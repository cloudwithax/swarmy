package ssefix

import (
	"bufio"
	"bytes"
	"io"

	"github.com/openai/openai-go/v2/packages/ssestream"
)

func init() {
	// Register a custom SSE decoder that filters out empty data events.
	// This fixes a bug where consecutive SSE comment lines (e.g., ": OPENROUTER PROCESSING")
	// cause the decoder to produce events with empty data, which then fail JSON unmarshaling
	// with "unexpected end of JSON input".
	ssestream.RegisterDecoder("text/event-stream", func(rc io.ReadCloser) ssestream.Decoder {
		scn := bufio.NewScanner(rc)
		scn.Buffer(nil, bufio.MaxScanTokenSize<<9)
		return &fixedEventStreamDecoder{
			rc:  rc,
			scn: scn,
		}
	})
}

type fixedEventStreamDecoder struct {
	evt ssestream.Event
	rc  io.ReadCloser
	scn *bufio.Scanner
	err error
}

func (s *fixedEventStreamDecoder) Next() bool {
	if s.err != nil {
		return false
	}

	event := ""
	data := bytes.NewBuffer(nil)

	for s.scn.Scan() {
		txt := s.scn.Bytes()

		// Dispatch event on an empty line
		if len(txt) == 0 {
			// Skip events with empty data (no data lines were processed)
			if data.Len() == 0 {
				continue
			}
			// Check if data is [DONE] marker - skip it
			dataBytes := data.Bytes()
			if bytes.Equal(bytes.TrimSpace(dataBytes), []byte("[DONE]")) {
				data.Reset()
				event = ""
				continue
			}
			s.evt = ssestream.Event{
				Type: event,
				Data: dataBytes,
			}
			return true
		}

		// Split a string like "event: bar" into name="event" and value=" bar".
		name, value, _ := bytes.Cut(txt, []byte(":"))

		// Consume an optional space after the colon if it exists.
		if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}

		switch string(name) {
		case "":
			// An empty name means this is a comment line (e.g., ": comment")
			// Comments should be ignored.
			continue
		case "event":
			event = string(value)
		case "data":
			_, s.err = data.Write(value)
			if s.err != nil {
				break
			}
			_, s.err = data.WriteRune('\n')
			if s.err != nil {
				break
			}
		}
	}

	if s.scn.Err() != nil {
		s.err = s.scn.Err()
	}

	return false
}

func (s *fixedEventStreamDecoder) Event() ssestream.Event {
	return s.evt
}

func (s *fixedEventStreamDecoder) Close() error {
	return s.rc.Close()
}

func (s *fixedEventStreamDecoder) Err() error {
	return s.err
}
