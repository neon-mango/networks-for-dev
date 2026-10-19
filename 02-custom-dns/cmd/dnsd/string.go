package main

import (
	"fmt"
	"strings"
)

func (id ResourceId) String() string {
	return fmt.Sprintf("%s type=%d class=%d",
		strings.Join(id.DomainName, "."), id.ResourceType, id.Class)
}

func (r Resource) String() string {
	// Сырые байты RDATA печатаются как hex.
	return fmt.Sprintf("%s ttl=%d data=%x", r.ResourceId, r.SecondsValid, r.Data)
}

func (h MessageHeader) String() string {
	return fmt.Sprintf("id=0x%04x isResponse=%t opCode=%d isAuthoritative=%t wasTruncated=%t"+
		" recursionDesired=%t recursionAvailable=%t wasVerified=%t"+
		" checkingDisabled=%t returnCode=%d",
		h.TransactionId, h.IsResponse, h.OpCode, h.IsAuthoritative, h.WasTruncated,
		h.RecursionDesired, h.RecursionAvailable, h.WasVerified,
		h.CheckingDisabled, h.ReturnCode)
}

// Флаги печатаются побитово, чтобы видеть упакованные поля.
func (h messageHeaderEncoded) String() string {
	return fmt.Sprintf("id=0x%04x flags=0b%016b", h.TransactionId, h.Flags)
}

func (l lengthsEncoded) String() string {
	return fmt.Sprintf("questions=%d answers=%d accessPerms=%d additionalRecords=%d",
		l.NumQuestions, l.NumAnswers, l.NumAccessPerms, l.NumAdditionalRecords)
}

func (m Message) String() string {
	var sb strings.Builder
	sb.WriteString(m.MessageHeader.String())
	fmt.Fprintf(&sb, "; questions=[%s]", joinResourceIds(m.Questions))
	fmt.Fprintf(&sb, "; answers=[%s]", joinResources(m.Answers))
	fmt.Fprintf(&sb, "; accessPerms=[%s]", joinResources(m.AccessPerms))
	fmt.Fprintf(&sb, "; additionalRecords=[%s]", joinResources(m.AdditionalRecords))
	return sb.String()
}

func joinResourceIds(ids []ResourceId) string {
	parts := make([]string, len(ids))
	for i := range ids {
		parts[i] = ids[i].String()
	}
	return strings.Join(parts, ", ")
}

func joinResources(resources []Resource) string {
	parts := make([]string, len(resources))
	for i := range resources {
		parts[i] = resources[i].String()
	}
	return strings.Join(parts, ", ")
}
