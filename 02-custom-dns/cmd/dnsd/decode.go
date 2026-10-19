package main

// Декодирование DNS-сообщений из пакета.

// https://www.rfc-editor.org/info/rfc1035/

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

func readMessageHeader(input io.Reader, msgHdr *MessageHeader) error {
	var msgHdrEncoded messageHeaderEncoded

	// Чтение
	err := binary.Read(input, binary.BigEndian, &msgHdrEncoded)
	if err != nil {
		return err
	}

	msgHdr.TransactionId = msgHdrEncoded.TransactionId

	// Разбор упакованных битов
	msgHdr.IsResponse = ((msgHdrEncoded.Flags >> 15) & 0x1) != 0
	msgHdr.OpCode = uint8((msgHdrEncoded.Flags >> 11) & 0xF)
	msgHdr.IsAuthoritative = ((msgHdrEncoded.Flags >> 10) & 0x1) != 0
	msgHdr.WasTruncated = ((msgHdrEncoded.Flags >> 9) & 0x1) != 0
	msgHdr.RecursionDesired = ((msgHdrEncoded.Flags >> 8) & 0x1) != 0
	msgHdr.RecursionAvailable = ((msgHdrEncoded.Flags >> 7) & 0x1) != 0
	msgHdr.WasVerified = ((msgHdrEncoded.Flags >> 5) & 0x1) != 0
	msgHdr.CheckingDisabled = ((msgHdrEncoded.Flags >> 4) & 0x1) != 0
	// Второй нулевой бит игнорируется, зарезервирован.
	msgHdr.ReturnCode = uint8(msgHdrEncoded.Flags & 0xF)

	// Проверки
	// TODO. Ошибки для значений вне диапазона.

	return nil
}

func readResourceId(input *bytes.Reader, resourceId *ResourceId) error {
	if err := readDomainName(input, &resourceId.DomainName); err != nil {
		return err
	}

	err := binary.Read(input, binary.BigEndian, &resourceId.ResourceType)
	if err != nil {
		return err
	}

	// Class обычно 1 (интернет).
	err = binary.Read(input, binary.BigEndian, &resourceId.Class)
	if err != nil {
		return err
	}

	// Проверки
	// TODO. Ошибки для значений вне диапазона; неизвестный тип записи, class != 1 (свои типы ошибок, чтобы писать ответ, если понадобится).
	// TODO: ошибка для пустого имени ресурса.

	return nil
}

// maxNameParts ограничивает число меток, чтобы зацикленный указатель
// сжатия не привёл к бесконечному чтению.
const maxNameParts = 128

func readDomainName(input *bytes.Reader, name *[]string) error {
	var labels []string
	returnTo := int64(-1)

	for len(labels) <= maxNameParts {
		var counter uint8
		err := binary.Read(input, binary.BigEndian, &counter)
		if err != nil {
			return err
		}

		switch {
		case counter == 0:
			*name = append(*name, labels...)
			if returnTo < 0 {
				return nil
			}

			_, err = input.Seek(returnTo, io.SeekStart)
			return err

		case counter>>6 == 0x3:
			// Указатель сжатия.
			var offsetLow uint8
			err = binary.Read(input, binary.BigEndian, &offsetLow)
			if err != nil {
				return err
			}

			offset := int64(uint64(counter&0x3F)<<8 | uint64(offsetLow))
			if returnTo < 0 {
				returnTo = input.Size() - int64(input.Len())
			}

			_, err = input.Seek(offset, io.SeekStart)
			if err != nil {
				return err
			}

		default:
			label := make([]byte, counter)
			_, err = io.ReadFull(input, label)
			if err != nil {
				return err
			}

			labels = append(labels, string(label))
		}
	}

	return errors.New("too many domain name parts")
}

func readResource(input *bytes.Reader, resource *Resource) error {
	err := readResourceId(input, &resource.ResourceId)
	if err != nil {
		return err
	}

	var secondsValid uint32 // TTL в секундах
	err = binary.Read(input, binary.BigEndian, &secondsValid)
	if err != nil {
		return err
	}

	var dataLength uint16
	err = binary.Read(input, binary.BigEndian, &dataLength)
	if err != nil {
		return err
	}

	// длина данных максимум 65 КБ, помнить об этом.
	// TODO: учитывать вредоносные запросы и перегрузку.
	data := make([]byte, dataLength)
	err = binary.Read(input, binary.BigEndian, data)
	if err != nil {
		return err
	}
	resource.Data = data

	return nil
}

func Decode(input []byte, msg *Message) error {
	reader := bytes.NewReader(input)

	err := readMessageHeader(reader, &msg.MessageHeader)
	if err != nil {
		return err
	}

	var lengths lengthsEncoded

	err = binary.Read(reader, binary.BigEndian, &lengths)
	if err != nil {
		return err
	}

	// fmt.Fprintf(os.Stderr, "NumQuestions = %d, NumAnswers = %d, NumAccessPerms = %d, NumAdditionalRecords = %d\n", lengths.NumQuestions, lengths.NumAnswers, lengths.NumAccessPerms, lengths.NumAdditionalRecords)

	for range lengths.NumQuestions {
		var resourceId ResourceId
		err = readResourceId(reader, &resourceId)
		if err != nil {
			return err
		}

		msg.Questions = append(msg.Questions, resourceId)
	}

	for range lengths.NumAnswers {
		var resource Resource
		err = readResource(reader, &resource)
		if err != nil {
			return err
		}

		msg.Answers = append(msg.Answers, resource)
	}

	for range lengths.NumAccessPerms {
		var resource Resource
		err = readResource(reader, &resource)
		if err != nil {
			return err
		}

		msg.AccessPerms = append(msg.AccessPerms, resource)
	}

	for range lengths.NumAdditionalRecords {
		var resource Resource
		err = readResource(reader, &resource)
		if err != nil {
			return err
		}

		msg.AdditionalRecords = append(msg.AdditionalRecords, resource)
	}

	return nil
}
