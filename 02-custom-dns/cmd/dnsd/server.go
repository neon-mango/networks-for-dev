package main

// Обработка запросов: цикл чтения, пересылка запросов на upstream,
// ожидающие могут переиспользовать уже отправленный запрос.

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

const (
	maxPacketSize   = 1024
	writeTimeout    = 1 * time.Second
	upstreamTimeout = 3 * time.Second
	waitTimeout     = 2 * upstreamTimeout
)

type server struct {
	conn       net.PacketConn
	upstream   string
	logger     *log.Logger
	queryBound chan struct{} // ограничивает число горутин с запросами от пользователя

	waitMu sync.Mutex
	waits  map[queryKey][]chan upstreamAnswer
}

func newServer(conn net.PacketConn, upstream string, logger *log.Logger, maxInflight int) *server {
	if maxInflight < 1 {
		maxInflight = 1
	}
	return &server{
		conn:       conn,
		upstream:   upstream,
		logger:     logger,
		queryBound: make(chan struct{}, maxInflight),
		waits:      make(map[queryKey][]chan upstreamAnswer),
	}
}

func (s *server) serve() {
	buffer := make([]byte, maxPacketSize)
	for {
		n, addr, err := s.conn.ReadFrom(buffer)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			s.logger.Println("Failed to read query", err)
			continue
		}

		packet := make([]byte, n)
		copy(packet, buffer[:n])

		select {
		case s.queryBound <- struct{}{}:
			go func() {
				defer func() { <-s.queryBound }()
				s.handleQuery(packet, addr)
			}()
		default:
			// Перегрузка: пакет отбрасывается, клиент попробует снова позже.
			s.logger.Println("Too many queries in flight, dropping a packet from", addr)
		}
	}
}

func (s *server) fillResponse(response *Message, packet []byte, addr net.Addr) {
	var query Message
	if err := Decode(packet, &query); err != nil {
		s.logger.Println("Failed to parse the packet from", addr, err)
		response.IsResponse = true
		response.ReturnCode = RETURN_CODE_FORMAT_ERROR
		return
	}

	response.MessageHeader = query.MessageHeader
	response.Questions = query.Questions
	response.IsResponse = true
	response.RecursionAvailable = true

	var upstreamQuestions []ResourceId
	var answers []Resource
	for _, question := range response.Questions {
		known, ok := lookupKnownResource(question)
		if !ok {
			upstreamQuestions = append(upstreamQuestions, question)
			continue
		}

		resource, err := known.toResource()
		if err != nil {
			// Запись настроена неверно: отдаём вопрос на upstream.
			s.logger.Println("Failed to encode the known resource", known, err)
			upstreamQuestions = append(upstreamQuestions, question)
			continue
		}

		answers = append(answers, resource)
	}

	response.Answers = answers
	if len(upstreamQuestions) == 0 {
		// Все вопросы отданы локально, upstream не опрашивается.
		response.IsAuthoritative = true
		response.ReturnCode = RETURN_CODE_NO_ERROR
	} else {
		// Смешанный запрос: на upstream пересылаются только неизвестные вопросы.
		encoded, err := Encode(&Message{
			MessageHeader: query.MessageHeader,
			Questions:     upstreamQuestions,
		})
		if err != nil {
			s.logger.Println("Failed to encode the upstream query", err)
			response.ReturnCode = RETURN_CODE_SERVFAIL
			return
		}

		s.forwardToUpstream(response, messageKey(upstreamQuestions), encoded)
	}

	for _, q := range query.Questions {
		fmt.Printf("%s: %s -> %d answers\n", addr, q, len(response.Answers))
	}
}

func (s *server) forwardToUpstream(response *Message, key queryKey, packet []byte) {
	result := s.fetchFromUpstream(key, packet)

	if result.err != nil {
		response.ReturnCode = RETURN_CODE_SERVFAIL
		return
	}

	response.ReturnCode = result.returnCode
	response.Answers = append(response.Answers, result.answers...)
}

func (s *server) handleQuery(packet []byte, addr net.Addr) {
	var response Message
	s.fillResponse(&response, packet, addr)

	out, err := Encode(&response)
	if err != nil {
		s.logger.Println("Failed to encode the response", err)
		return
	}

	s.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if _, err := s.conn.WriteTo(out, addr); err != nil {
		s.logger.Println("Failed to write response", err)
		return
	}
}

func (s *server) fetchFromUpstream(key queryKey, packet []byte) upstreamAnswer {
	needFetch, ch := s.registerWaiter(key)
	defer s.releaseWaiter(key, ch)
	if needFetch {
		result := s.exchange(packet)
		s.deliverWaiters(key, result)
	}

	timer := time.NewTimer(waitTimeout)
	defer timer.Stop()
	select {
	case result := <-ch:
		return result
	case <-timer.C:
		return upstreamAnswer{err: errors.New("timed out waiting for the upstream lookup")}
	}
}

// exchange пересылает пакет на upstream как есть и читает ответ обратно.
func (s *server) exchange(packet []byte) upstreamAnswer {
	conn, err := net.Dial("udp4", s.upstream)
	if err != nil {
		return upstreamAnswer{err: err}
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(upstreamTimeout)); err != nil {
		return upstreamAnswer{err: err}
	}
	if _, err := conn.Write(packet); err != nil {
		return upstreamAnswer{err: err}
	}
	response := make([]byte, maxPacketSize)
	n, err := conn.Read(response)
	if err != nil {
		return upstreamAnswer{err: err}
	}
	var msg Message
	if err := Decode(response[:n], &msg); err != nil {
		return upstreamAnswer{err: err}
	}
	return upstreamAnswer{answers: msg.Answers, returnCode: msg.ReturnCode}
}

type upstreamAnswer struct {
	answers    []Resource // записи-ответы из ответа upstream
	returnCode uint8      // код ответа из заголовка upstream
	err        error      // сбой транспорта, nil при успехе
}

func (s *server) registerWaiter(key queryKey) (needFetch bool, ch chan upstreamAnswer) {
	s.waitMu.Lock()
	defer s.waitMu.Unlock()
	ch = make(chan upstreamAnswer, 1)
	s.waits[key] = append(s.waits[key], ch)
	return len(s.waits[key]) == 1, ch
}

func (s *server) deliverWaiters(key queryKey, result upstreamAnswer) {
	s.waitMu.Lock()
	defer s.waitMu.Unlock()
	for _, ch := range s.waits[key] {
		ch <- result // буферизованный, не блокирует
		close(ch)
	}
	delete(s.waits, key)
}

// releaseWaiter снимает одного ожидающего ресурс, убирает канал.
// Ключ удаляется, когда больше не нужен.
func (s *server) releaseWaiter(key queryKey, ch chan upstreamAnswer) {
	s.waitMu.Lock()
	defer s.waitMu.Unlock()
	waiters := s.waits[key]
	for i, waiter := range waiters {
		if waiter == ch {
			s.waits[key] = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
	if len(s.waits[key]) == 0 {
		delete(s.waits, key)
	}
}
