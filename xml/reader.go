package xml

import (
	"encoding/xml"
	"io"
	"os"
)

// converts a file to records ((data, error) tuples)
type Reader struct {
	xmlFile	*os.File
	decoder *xml.Decoder
}

type Record struct {
	TypeName 	string
	Data		interface{}
}

type RecordsBuilderResult struct {
	Records	[]*Record
	Err		error
}

type ProcessTokenResult struct {
	Records       []*Record
	IsEndOfStream bool
	Err           error
}

type RecordsBuilderFunction func(xml.Token) RecordsBuilderResult

func (r *Reader) Open(filename string) error {
	var err error
	r.xmlFile, err = os.Open(filename)

	if err != nil {
		return err
	}

	r.decoder = xml.NewDecoder(r.xmlFile)

	return nil
}

func (r *Reader) BuildRecordsFromToken(recordsBuilder RecordsBuilderFunction) ProcessTokenResult {

	// decode a token
	t, err := r.decoder.Token()

	// return an error, if one happened
	if err != nil {
		if err == io.EOF {
			return ProcessTokenResult{nil, true, nil}
		}

		return ProcessTokenResult{nil, false, err}
	}

	// stop looping when we have no more tokens
	if t == nil {
		return ProcessTokenResult{nil, true, nil}
	}

	res := recordsBuilder(t)
	return ProcessTokenResult{res.Records, false, res.Err}
}

func (r *Reader) DecodeToken(v interface{}, start *xml.StartElement) error {
	return r.decoder.DecodeElement(v, start)
}
