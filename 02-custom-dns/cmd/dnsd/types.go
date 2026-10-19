package main

// https://creewick.github.io/study/courses/inet/notes/dns/format.html

import (
	"slices"
	"strconv"
	"strings"
)

const (
	OpQuery  = 0
	OpIQuery = 1
	OpStatus = 2
)

const (
	RESOURCE_TYPE_A     = 1   // IPv4
	RESOURCE_TYPE_AAAA  = 28  // IPv6
	RESOURCE_TYPE_AXFR  = 252 // Запрос на передачу зоны
	RESOURCE_TYPE_CNAME = 5   // Каноничное имя (TODO: что это?)
	RESOURCE_TYPE_HINFO = 13  // Информация о хосте (CPU, OS)
	RESOURCE_TYPE_MX    = 15  // Запись об обмене почтой (куда слать почту)
	RESOURCE_TYPE_NS    = 2   // Сервер DNS (TODO: для доменных зон?)
	RESOURCE_TYPE_PTR   = 12  // Запись указателя (TODO: что это такое?)
	RESOURCE_TYPE_ANY   = 255 // Все записи
)

const (
	CLASS_INTERNET = 1 // интернет
	CLASS_CSNET    = 2 // класс CSNET (устарел, встречается только в примерах некоторых RFC)
	CLASS_CHAOS    = 3 // класс CHAOS (для chaosnet, не получившего распространение)
	CLASS_HESIOD   = 4 // Hesiod (проект Athena)
)

// https://www.rfc-editor.org/info/rfc1035/#section-4.1.1, см. "RCODE"
const (
	RETURN_CODE_NO_ERROR      = 0
	RETURN_CODE_FORMAT_ERROR  = 1
	RETURN_CODE_SERVFAIL      = 2
	RETURN_CODE_NAME_ERROR    = 3
	RETURN_CODE_UNIMPLEMENTED = 4
	RETURN_CODE_REFUSED       = 5
)

type ResourceId struct {
	DomainName   []string
	ResourceType uint16
	Class        uint16 // Обычно 1 - интернет.
}

// queryKey - хешируемое представление вопросов сообщения:
// напрямую ключом map их сделать нельзя из-за полей-срезов ([] smth).
// Имена доменов сравниваются без учёта регистра.
type queryKey string

// normalizeDomainName копирует имя, приводя каждую часть к нижнему регистру,
// так что имена сравниваются без учёта регистра, как в DNS.
func normalizeDomainName(domainName []string) []string {
	result := make([]string, len(domainName))
	for i, part := range domainName {
		result[i] = strings.ToLower(part)
	}

	return result
}

// equalResourceIds сообщает, одинаковы ли два id ресурсов.
func equalResourceIds(a, b ResourceId) bool {
	return a.ResourceType == b.ResourceType &&
		a.Class == b.Class &&
		slices.Equal(normalizeDomainName(a.DomainName), normalizeDomainName(b.DomainName))
}

func messageKey(questions []ResourceId) queryKey {
	parts := make([]string, len(questions))
	for i, q := range questions {
		parts[i] = strings.Join(normalizeDomainName(q.DomainName), "\x00") +
			"\x00" + strconv.FormatUint(uint64(q.ResourceType), 10) +
			"\x00" + strconv.FormatUint(uint64(q.Class), 10)
	}
	return queryKey(strings.Join(parts, "\x01"))
}

// ResourceRecord назван как Resource для краткости
type Resource struct {
	ResourceId
	SecondsValid uint32
	Data         []byte
}

// Resource с типизированными данными.
type KnownResource struct {
	ResourceId
	SecondsValid uint32
	Data         any // RecordA, CannonicalName или HardwareInfo
}

func (k KnownResource) toResource() (Resource, error) {
	data, err := rdataBytes(k.Data)
	if err != nil {
		return Resource{}, err
	}

	return Resource{ResourceId: k.ResourceId, SecondsValid: k.SecondsValid, Data: data}, nil
}

type MessageHeader struct {
	TransactionId      uint16
	IsResponse         bool // QR флаг: ложь для запроса, истина для ответа
	OpCode             uint8
	IsAuthoritative    bool
	WasTruncated       bool
	RecursionDesired   bool
	RecursionAvailable bool
	WasVerified        bool
	CheckingDisabled   bool
	ReturnCode         uint8
}

type messageHeaderEncoded struct {
	TransactionId uint16
	Flags         uint16
}

type lengthsEncoded struct {
	NumQuestions         uint16
	NumAnswers           uint16
	NumAccessPerms       uint16
	NumAdditionalRecords uint16
}

type Message struct {
	MessageHeader
	Questions         []ResourceId
	Answers           []Resource
	AccessPerms       []Resource
	AdditionalRecords []Resource
}

// Поддерживаемые типы записей

// https://github.com/bluelibraries/dns/blob/main/docs/Records/cname.md

type HardwareInfo struct {
	CPU             string
	OperatingSystem string
}

type CannonicalName struct {
	host string
}

type RecordA struct {
	ipv4 string
}
