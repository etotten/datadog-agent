// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2018 Datadog, Inc.

package flare

import (
	"bufio"
	"errors"
	"io/ioutil"
	"os"

	log "github.com/cihub/seelog"
)

//RedactingWriter is a writer that will redact content before writing to target
type RedactingWriter struct {
	target    *os.File
	targetBuf *bufio.Writer
	perm      os.FileMode
	r         []replacer
}

//NewRedactingWriter instantiates a RedactingWriter to target with given permissions
func NewRedactingWriter(t string, p os.FileMode, buffered bool) (*RedactingWriter, error) {
	err := ensureParentDirsExist(t)
	if err != nil {
		return nil, err
	}

	f, err := os.Create(t)
	if err != nil {
		return nil, err
	}

	var b *bufio.Writer
	if buffered {
		b = bufio.NewWriter(f)
	}

	return &RedactingWriter{
		target:    f,
		targetBuf: b,
		perm:      p,
		r:         []replacer{},
	}, nil
}

//RegisterReplacer register additional replacers to run on stream
func (f *RedactingWriter) RegisterReplacer(r replacer) {
	f.r = append(f.r, r)
}

//WriteFromFile will read contents from file and write them redacted to target
func (f *RedactingWriter) WriteFromFile(filePath string) (int, error) {

	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			log.Warnf("the specified path: %s does not exist", filePath)
		}

		return 0, err
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return 0, err
	}

	return f.Write(data)
}

//Write writes the redacted byte stream, applying all replacers and credential
//cleanup to target
func (f *RedactingWriter) Write(p []byte) (int, error) {
	fReady, buffered := (f.target != nil), (f.targetBuf != nil)

	if !fReady && !buffered {
		return 0, errors.New("No viable target defined")
	}

	cleaned, err := credentialsCleanerBytes(p)
	if err != nil {
		return 0, err
	}

	for _, r := range f.r {
		if r.regex != nil && r.replFunc != nil {
			cleaned = r.regex.ReplaceAllFunc(cleaned, r.replFunc)
		}
	}

	var n int
	if buffered {
		n, err = f.targetBuf.Write(cleaned)
	} else {
		n, err = f.target.Write(cleaned)
	}

	return n, err
}

//Flush if this is a buffered writer, it flushes the buffer, otherwise NOP
func (f *RedactingWriter) Flush() error {

	if f.targetBuf == nil {
		return nil
	}

	return f.targetBuf.Flush()
}

//Close closes the underlying file, if buffered previously flushes the contents
func (f *RedactingWriter) Close() error {
	var err error

	if f.targetBuf != nil {
		err = f.targetBuf.Flush()
		if err != nil {
			return err
		}
	}

	if f.target != nil {
		err = f.target.Close()
	}

	return err
}
