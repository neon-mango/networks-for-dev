package main

// Кодирование DNS-сообщений в wire-формат.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
)

func writeMessageHeader(output io.Writer, msgHdr *MessageHeader) error {
	var msgHdrEncoded messageHeaderEncoded

	msgHdrEncoded.TransactionId = msgHdr.TransactionId

	// Проверка.
	// TODO. Ошибки для значений вне диапазона.
	if msgHdr.OpCode > 0xF {
		return errors.New("opCode does not fit into 4 bits")
	}
	if msgHdr.ReturnCode > 0xF {
		return errors.New("returnCode does not fit into 4 bits")
	}

	// Запись упакованных битов, см. RFC 1035, 4.1.1.
	//
	// Флаги идут от старшего бита к младшим: QR OPCODE AA TC RD RA Z AD CD RCODE
	// (AD и CD определены в RFC 4035).
	if msgHdr.IsResponse {
		msgHdrEncoded.Flags |= 1 << 15
	}

	msgHdrEncoded.Flags |= uint16(msgHdr.OpCode) << 11

	if msgHdr.IsAuthoritative {
		msgHdrEncoded.Flags |= 1 << 10
	}

	if msgHdr.WasTruncated {
		msgHdrEncoded.Flags |= 1 << 9
	}

	if msgHdr.RecursionDesired {
		msgHdrEncoded.Flags |= 1 << 8
	}

	if msgHdr.RecursionAvailable {
		msgHdrEncoded.Flags |= 1 << 7
	}

	if msgHdr.WasVerified {
		msgHdrEncoded.Flags |= 1 << 5
	}

	if msgHdr.CheckingDisabled {
		msgHdrEncoded.Flags |= 1 << 4
	}

	msgHdrEncoded.Flags |= uint16(msgHdr.ReturnCode)

	// Запись
	err := binary.Write(output, binary.BigEndian, &msgHdrEncoded)
	if err != nil {
		return err
	}

	return nil
}

func writeDomainName(output io.Writer, labels []string) error {
	for _, label := range labels {
		// Пустая метка декодировалась бы как завершение, ломало имя.
		if len(label) == 0 {
			return errors.New("empty domain name part")
		}

		// Метки ограничены 63 байтами в DNS.
		if len(label) > 63 {
			return errors.New("domain name part is longer than 63 bytes")
		}

		counter := uint8(len(label))
		err := binary.Write(output, binary.BigEndian, &counter)
		if err != nil {
			return err
		}

		err = binary.Write(output, binary.BigEndian, []byte(label))
		if err != nil {
			return err
		}
	}

	// Нулевой байт завершает имя.
	var terminator uint8
	err := binary.Write(output, binary.BigEndian, &terminator)
	if err != nil {
		return err
	}

	return nil
}

func writeResourceId(output io.Writer, resourceId *ResourceId) error {
	// Запись доменного имени.
	err := writeDomainName(output, resourceId.DomainName)
	if err != nil {
		return err
	}

	// Запись остальных полей

	err = binary.Write(output, binary.BigEndian, &resourceId.ResourceType)
	if err != nil {
		return err
	}

	// Class обычно 1 (интернет).

	err = binary.Write(output, binary.BigEndian, &resourceId.Class)
	if err != nil {
		return err
	}

	// Проверки
	// TODO. Выдавать ошибки для значений вне диапазона; неизвестный тип записи, class != 1 (свои типы ошибок, чтобы писать ответ, если понадобится).

	return nil
}

func writeResource(output io.Writer, resource *Resource) error {
	err := writeResourceId(output, &resource.ResourceId)
	if err != nil {
		return err
	}

	// SecondsValid - это TTL в секундах.
	err = binary.Write(output, binary.BigEndian, &resource.SecondsValid)
	if err != nil {
		return err
	}

	// длина данных максимум 65 КБ, помнить об этом.
	if len(resource.Data) > math.MaxUint16 {
		return errors.New("resource data is longer than 65KB")
	}

	dataLength := uint16(len(resource.Data))
	err = binary.Write(output, binary.BigEndian, &dataLength)
	if err != nil {
		return err
	}

	// TODO: учитывать вредоносные запросы и перегрузку.
	err = binary.Write(output, binary.BigEndian, resource.Data)
	if err != nil {
		return err
	}

	return nil
}

func rdataBytes(data any) ([]byte, error) {
	var buf bytes.Buffer

	switch record := data.(type) {
	case RecordA:
		ip := net.ParseIP(record.ipv4)
		if ip == nil {
			return nil, fmt.Errorf("%q is not an IP address", record.ipv4)
		}

		ipv4 := ip.To4()
		if ipv4 == nil {
			return nil, fmt.Errorf("%q is not an IPv4 address", record.ipv4)
		}

		buf.Write(ipv4)
	case CannonicalName:
		// RDATA CNAME - доменное имя, записывается как есть.
		if record.host == "" {
			return nil, errors.New("empty canonical name")
		}

		err := writeDomainName(&buf, strings.Split(record.host, "."))
		if err != nil {
			return nil, err
		}
	case HardwareInfo:
		// RDATA HINFO - две символьные строки, см. RFC 1035, 3.3.2.
		for _, s := range []string{record.CPU, record.OperatingSystem} {
			if err := writeCharacterString(&buf, s); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("unsupported RDATA type %T", data)
	}

	return buf.Bytes(), nil
}

// writeCharacterString записывает символьные строки с префиксом длины,
// см. RFC 1035, 3.3.
func writeCharacterString(output io.Writer, s string) error {
	if len(s) > 255 {
		return errors.New("character-string is longer than 255 bytes")
	}

	length := uint8(len(s))
	err := binary.Write(output, binary.BigEndian, &length)
	if err != nil {
		return err
	}

	_, err = output.Write([]byte(s))
	return err
}

func Encode(msg *Message) ([]byte, error) {
	// Проверки
	// TODO. Выдавать ошибки для значений вне диапазона.

	tooLong := len(msg.Questions) > math.MaxUint16 ||
		len(msg.Answers) > math.MaxUint16 ||
		len(msg.AccessPerms) > math.MaxUint16 ||
		len(msg.AdditionalRecords) > math.MaxUint16
	if tooLong {
		return nil, errors.New("too many entries in the message")
	}

	var buf bytes.Buffer

	err := writeMessageHeader(&buf, &msg.MessageHeader)
	if err != nil {
		return nil, err
	}

	lengths := lengthsEncoded{
		NumQuestions:         uint16(len(msg.Questions)),
		NumAnswers:           uint16(len(msg.Answers)),
		NumAccessPerms:       uint16(len(msg.AccessPerms)),
		NumAdditionalRecords: uint16(len(msg.AdditionalRecords)),
	}

	err = binary.Write(&buf, binary.BigEndian, &lengths)
	if err != nil {
		return nil, err
	}

	for i := range msg.Questions {
		err = writeResourceId(&buf, &msg.Questions[i])
		if err != nil {
			return nil, err
		}
	}

	for i := range msg.Answers {
		err = writeResource(&buf, &msg.Answers[i])
		if err != nil {
			return nil, err
		}
	}

	for i := range msg.AccessPerms {
		err = writeResource(&buf, &msg.AccessPerms[i])
		if err != nil {
			return nil, err
		}
	}

	for i := range msg.AdditionalRecords {
		err = writeResource(&buf, &msg.AdditionalRecords[i])
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}
