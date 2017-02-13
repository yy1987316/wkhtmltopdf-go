package wkhtmltopdf

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
)

// A Document represents a single pdf document.
type Document struct {
	cover   *Page
	pages   []*Page
	options []string

	tmp string // temp directory
}

// NewDocument creates a new document.
func NewDocument(opts ...Option) *Document {

	doc := &Document{pages: []*Page{}, options: []string{}}
	doc.AddOptions(opts...)
	return doc
}

// AddPages to the document. Pages will be included in
// the final pdf in the order they are added.
func (doc *Document) AddPages(pages ...*Page) {
	doc.pages = append(doc.pages, pages...)
}

// AddCover adds a cover page to the document.
func (doc *Document) AddCover(cover *Page) {
	doc.cover = cover
}

// AddOptions allows the setting of options after document creation.
func (doc *Document) AddOptions(opts ...Option) {

	for _, opt := range opts {
		doc.options = append(doc.options, opt.opts()...)
	}
}

// args calculates the args needed to run wkhtmltopdf
func (doc *Document) args() []string {

	args := []string{}
	args = append(args, doc.options...)

	// coverpage
	if doc.cover != nil {
		args = append(args, "cover", doc.cover.filename)
		args = append(args, doc.cover.options...)
	}

	// pages
	for _, pg := range doc.pages {
		args = append(args, pg.filename)
		args = append(args, pg.options...)
	}

	return args
}

// readers counts the number of pages using a reader
// as a source
func (doc *Document) readers() int {

	n := 0
	if doc.cover != nil && doc.cover.reader {
		n++
	}

	for _, pg := range doc.pages {
		if pg.reader {
			n++
		}
	}
	return n
}

// writeTempPages writes the pages generated by a reader
// to a set of pages within a temp directory.
func (doc *Document) writeTempPages() error {

	var err error
	doc.tmp, err = ioutil.TempDir(TempDir, "temp")
	if err != nil {
		return fmt.Errorf("Error creating temp directory")
	}

	n := 0
	all_pages := []*Page{}
	if doc.cover != nil {
		all_pages = append(all_pages, doc.cover)
	}
	all_pages = append(all_pages, doc.pages...)
	for _, pg := range all_pages {
		if !pg.reader {
			continue
		}

		n++
		pg.filename = fmt.Sprintf("%v/%v/page%08d.html", TempDir, doc.tmp, n)
		err := ioutil.WriteFile(pg.filename, pg.buf.Bytes(), 0666)
		if err != nil {
			return fmt.Errorf("Error writing temp file: %v", err)
		}
	}

	return nil
}

// createPDF creates the pdf and writes it to the buffer,
// which can then be written to file or writer.
func (doc *Document) createPDF() (*bytes.Buffer, error) {

	var stdin io.Reader
	switch {
	case doc.readers() == 1:

		// Pipe through stdin for a single reader.
		for _, pg := range doc.pages {
			if pg.reader {
				stdin = pg.buf
				pg.filename = "-"
				break
			}
		}

	case doc.readers() > 1:

		// Write multiple readers to temp files
		err := doc.writeTempPages()
		if err != nil {
			return nil, fmt.Errorf("Error writing temp files: %v", err)
		}
	}

	args := append(doc.args(), "-")

	buf := &bytes.Buffer{}
	errbuf := &bytes.Buffer{}

	cmd := exec.Command(Executable, args...)
	cmd.Stdin = stdin
	cmd.Stdout = buf
	cmd.Stderr = errbuf

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("Error running wkhtmltopdf: %v", errbuf.String())
	}

	if doc.tmp != "" {
		err = os.RemoveAll(TempDir + "/" + doc.tmp)
	}
	return buf, err

}

// WriteToFile creates the pdf document and writes it
// to the specified filename.
func (doc *Document) WriteToFile(filename string) error {

	buf, err := doc.createPDF()
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filename, buf.Bytes(), 0666)
	if err != nil {
		return fmt.Errorf("Error creating file: %v", err)
	}

	return nil
}

// Write creates the pdf document and writes it
// to the provided reader.
func (doc *Document) Write(w io.Writer) error {

	buf, err := doc.createPDF()
	if err != nil {
		return err
	}

	_, err = buf.WriteTo(w)
	if err != nil {
		return fmt.Errorf("Error writing to writer: %v", err)
	}

	return nil
}
