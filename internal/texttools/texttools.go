// Package texttools holds the Text Editor's text tools (Tools menu, owner
// request v1.21.53): line tools, encoders and decoders, formatters and the
// readable copy of a SonicWall settings export. Every tool exists once, here;
// the page sends page-held text to the engine and the engine runs the same
// code over a buffer snapshot of a disk-backed document.
//
// A tool reads its input once as a stream (UTF-8 text in which every line
// break is one '\n', as the editor holds it) and reports what it does through
// Edits: the input bytes it keeps, the bytes it drops and the new text it
// inserts, in input order. Keeping a byte keeps it exactly, so a document
// whose lines end in different ways keeps the ending of every line a tool did
// not change. Tools that rewrite their whole input (a sort, a decoder, a
// formatter) drop it and insert the result.
//
// Memory does not grow with the input: tools stream, and the line tools that
// must compare lines (sort, remove duplicates, reverse) spill sorted runs to
// files in a scratch folder (lines.go). The only data held whole is one line
// where lines are compared, and the open element names of an XML document.
package texttools

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// Edits receives, in input order, what a tool does with its input.
type Edits interface {
	// Keep leaves the next n input bytes as they are.
	Keep(n int64) error
	// Drop removes the next n input bytes.
	Drop(n int64) error
	// Insert adds new text at the current input position.
	Insert(p []byte) error
}

// Source is the input of a tool. Open may be called more than once; each
// reader returns the same Size bytes from the start.
type Source interface {
	Open() (io.Reader, error)
	Size() int64
}

// Options are the choices a tool asks for. The page sends them as JSON.
type Options struct {
	// Separator is the text Split at text cuts at, or the text Join lines
	// puts between lines.
	Separator string `json:"separator"`
	// KeepSeparator leaves the separator at the end of each line (Split).
	KeepSeparator bool `json:"keepSeparator"`
	// Indent is the editor's indent: 0 for a tab, otherwise that many
	// spaces. It is the tab width of Tabs to spaces and Leading spaces to tabs.
	Indent int `json:"indent"`
}

// Result says what a tool did.
type Result struct {
	// Changed is false when the tool left its input as it was.
	Changed bool `json:"changed"`
	// Count is what the tool counts: lines sorted or removed, places split
	// or joined, settings written. Zero when it counts nothing.
	Count int64 `json:"count"`
}

// Env is what the caller provides besides the input.
type Env struct {
	// Dir is where sorted runs are spilled ("" for the system folder).
	Dir string
	// Progress, when set, receives how much of the work is done (0 to 1).
	Progress func(float64)
	// MaxOutput, when set, bounds the result in bytes (page-held text, whose
	// result the engine holds in memory and sends to the page).
	MaxOutput int64
}

// ErrTooLarge stops a tool whose result would pass Env.MaxOutput.
var ErrTooLarge = errors.New("The result would be too large to send to the page, so the tool stopped.")

// boundedEdits stops a tool whose result passes a size.
type boundedEdits struct {
	Edits
	max, size int64
}

func (b *boundedEdits) Keep(n int64) error {
	if b.size += n; b.size > b.max {
		return ErrTooLarge
	}
	return b.Edits.Keep(n)
}

func (b *boundedEdits) Insert(p []byte) error {
	if b.size += int64(len(p)); b.size > b.max {
		return ErrTooLarge
	}
	return b.Edits.Insert(p)
}

// Tool describes one tool for the registry.
type Tool struct {
	ID string
	// Lines is set for tools that work on whole lines: the page extends a
	// partial selection to the lines it touches before running them.
	Lines bool
	// Rewrite is set for tools that replace their whole input with a result
	// (a sort, a decoder, a formatter). Their result is compared with the
	// input to tell whether anything changed.
	Rewrite bool
	run     func(*job) error
}

// PositionError names where in the input a tool found a problem: Offset is
// in UTF-16 units from the start of the input, the unit the editor counts
// positions in. The caller turns it into a line and column of the document.
type PositionError struct {
	Offset  int64
	Message string
}

func (e *PositionError) Error() string { return e.Message }

// ErrUnknownTool answers a tool name that is not in the registry.
var ErrUnknownTool = errors.New("This text tool is not available.")

var registry = map[string]*Tool{}

func register(t *Tool) {
	if registry[t.ID] != nil {
		panic("text tool registered twice: " + t.ID)
	}
	registry[t.ID] = t
}

// Lookup returns the tool with this name.
func Lookup(id string) (*Tool, bool) {
	t, ok := registry[id]
	return t, ok
}

// IDs lists every registered tool, for the test that keeps the page's list
// (web/assets/editor-text-tools.js) and this one the same.
func IDs() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	return out
}

// Run runs a tool over src and reports what it does to out. When the result
// says nothing changed, out may have received only part of the edits (or
// none): the caller discards them.
func Run(ctx context.Context, id string, opts Options, src Source, out Edits, env Env) (Result, error) {
	tool, ok := registry[id]
	if !ok {
		return Result{}, ErrUnknownTool
	}
	if opts.Indent < 0 || opts.Indent > 16 {
		return Result{}, errors.New("Choose an indent of 0 (tabs) to 16 spaces.")
	}
	r, err := src.Open()
	if err != nil {
		return Result{}, err
	}
	j := &job{ctx: ctx, opts: opts, src: src, env: env, size: src.Size()}
	j.in = newReader(ctx, r, j.progress)
	if env.MaxOutput > 0 {
		out = &boundedEdits{Edits: out, max: env.MaxOutput}
	}
	if tool.Rewrite {
		compare, err := src.Open()
		if err != nil {
			return Result{}, err
		}
		j.rewrite = &rewriter{edits: out, same: bufio.NewReaderSize(compare, 32<<10), size: j.size}
		j.out = &emitter{ctx: ctx, out: j.rewrite}
	} else {
		j.out = &emitter{ctx: ctx, out: out}
	}
	if err = tool.run(j); err != nil {
		return Result{}, err
	}
	if j.same {
		// The tool found nothing to change and reported nothing.
		if env.Progress != nil {
			env.Progress(1)
		}
		return Result{}, nil
	}
	if err = j.out.finish(); err != nil {
		return Result{}, err
	}
	if j.in.off != j.size && !tool.Rewrite {
		return Result{}, fmt.Errorf("text tool %s read %d of %d bytes", id, j.in.off, j.size)
	}
	result := Result{Changed: j.out.changed, Count: j.count}
	if tool.Rewrite {
		changed, err := j.rewrite.finish()
		if err != nil {
			return Result{}, err
		}
		result.Changed = changed
	}
	if !result.Changed {
		result.Count = 0
	}
	if env.Progress != nil {
		env.Progress(1)
	}
	return result, nil
}

// job is one run of a tool.
type job struct {
	ctx     context.Context
	opts    Options
	src     Source
	env     Env
	size    int64
	in      *reader
	out     *emitter
	rewrite *rewriter
	count   int64
	// same is set by a tool that found its input already as it would make it
	// (sorted, without duplicates); it reports no edits at all then.
	same bool
	// passes scales progress for tools that read their input more than once.
	passes, pass int
}

func (j *job) progress(done int64) {
	if j.env.Progress == nil || j.size == 0 {
		return
	}
	passes := max(j.passes, 1)
	j.env.Progress((float64(j.pass) + float64(done)/float64(j.size)) / float64(passes))
}

// reopen starts another pass over the input.
func (j *job) reopen() error {
	r, err := j.src.Open()
	if err != nil {
		return err
	}
	j.pass++
	j.in = newReader(j.ctx, r, j.progress)
	return nil
}

func (j *job) indent() int {
	if j.opts.Indent <= 0 {
		return 4
	}
	return j.opts.Indent
}

// emitter merges the edits of a tool: runs of kept bytes become one Keep,
// and the drops and insertions between two kept runs become one Drop and one
// Insert (dropping and inserting at the same place commute).
type emitter struct {
	// ctx is asked for every block of new text: a tool that writes much
	// more than it reads stops promptly when cancelled.
	ctx     context.Context
	out     Edits
	keep    int64
	drop    int64
	insert  []byte
	changed bool
}

func (e *emitter) Keep(n int64) error {
	if n <= 0 {
		return nil
	}
	if e.drop > 0 || len(e.insert) > 0 {
		if err := e.flushChange(); err != nil {
			return err
		}
	}
	e.keep += n
	return nil
}

func (e *emitter) Drop(n int64) error {
	if n <= 0 {
		return nil
	}
	if err := e.flushKeep(); err != nil {
		return err
	}
	e.drop += n
	e.changed = true
	return nil
}

func (e *emitter) Insert(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	if err := e.flushKeep(); err != nil {
		return err
	}
	e.insert = append(e.insert, p...)
	e.changed = true
	if len(e.insert) >= 64<<10 {
		return e.flushChange()
	}
	return nil
}

func (e *emitter) InsertString(s string) error { return e.Insert([]byte(s)) }

func (e *emitter) flushKeep() error {
	if e.keep == 0 {
		return nil
	}
	n := e.keep
	e.keep = 0
	return e.out.Keep(n)
}

func (e *emitter) flushChange() error {
	if e.drop > 0 {
		n := e.drop
		e.drop = 0
		if err := e.out.Drop(n); err != nil {
			return err
		}
	}
	if len(e.insert) > 0 {
		if e.ctx != nil {
			if err := e.ctx.Err(); err != nil {
				return err
			}
		}
		p := e.insert
		e.insert = e.insert[:0]
		if err := e.out.Insert(p); err != nil {
			return err
		}
	}
	return nil
}

func (e *emitter) finish() error {
	if err := e.flushKeep(); err != nil {
		return err
	}
	return e.flushChange()
}

// rewriter is the Edits of a tool that rewrites its whole input. The tool
// keeps nothing: its result is inserted and the input dropped. The result is
// compared with the input on the way, so a result that equals its input is
// reported as no change.
type rewriter struct {
	edits    Edits
	same     *bufio.Reader
	size     int64
	written  int64
	differ   bool
	started  bool
	finished bool
}

func (w *rewriter) Keep(n int64) error { return errors.New("a rewriting text tool kept input") }
func (w *rewriter) Drop(n int64) error { return errors.New("a rewriting text tool dropped input") }

func (w *rewriter) Insert(p []byte) error {
	if !w.started {
		// The whole input goes; the result takes its place.
		w.started = true
		if w.size > 0 {
			if err := w.edits.Drop(w.size); err != nil {
				return err
			}
		}
	}
	if !w.differ {
		if w.written+int64(len(p)) > w.size {
			w.differ = true
		} else {
			buf := make([]byte, len(p))
			if _, err := io.ReadFull(w.same, buf); err != nil || string(buf) != string(p) {
				w.differ = true
			}
		}
	}
	w.written += int64(len(p))
	return w.edits.Insert(p)
}

// finish reports whether the result differs from the input.
func (w *rewriter) finish() (bool, error) {
	if !w.started && w.size > 0 {
		w.started = true
		if err := w.edits.Drop(w.size); err != nil {
			return false, err
		}
	}
	return w.differ || w.written != w.size, nil
}

// reader reads the input of a tool one character or byte at a time and
// counts where it is: bytes for Edits, UTF-16 units for error positions.
type reader struct {
	ctx      context.Context
	r        *bufio.Reader
	off      int64 // bytes read
	units    int64 // UTF-16 units read
	checked  int64
	progress func(int64)
}

func newReader(ctx context.Context, r io.Reader, progress func(int64)) *reader {
	return &reader{ctx: ctx, r: bufio.NewReaderSize(r, 64<<10), progress: progress}
}

const checkEvery = 256 << 10

func (r *reader) tick() error {
	if r.off-r.checked < checkEvery {
		return nil
	}
	r.checked = r.off
	if r.progress != nil {
		r.progress(r.off)
	}
	return r.ctx.Err()
}

// next returns the next character and its size in bytes; io.EOF at the end.
func (r *reader) next() (rune, int, error) {
	c, size, err := r.r.ReadRune()
	if err != nil {
		return 0, 0, err
	}
	if c == utf8.RuneError && size == 1 {
		return 0, 0, errors.New("the text is not valid UTF-8")
	}
	r.off += int64(size)
	r.units++
	if c > 0xffff {
		r.units++
	}
	return c, size, r.tick()
}

// byte returns the next byte; io.EOF at the end.
func (r *reader) byte() (byte, error) {
	c, err := r.r.ReadByte()
	if err != nil {
		return 0, err
	}
	r.off++
	if c < 0x80 || c >= 0xc0 {
		r.units++
		if c >= 0xf0 {
			r.units++
		}
	}
	return c, r.tick()
}

// peek returns up to n of the next bytes without reading them.
func (r *reader) peek(n int) []byte {
	p, _ := r.r.Peek(n)
	return p
}

// at is an error at the current position (just read).
func (r *reader) at(units int64, format string, args ...any) error {
	return &PositionError{Offset: units, Message: fmt.Sprintf(format, args...)}
}

// BytesSource is the input of page-held text.
type BytesSource []byte

func (b BytesSource) Open() (io.Reader, error) { return bytesReader(b), nil }
func (b BytesSource) Size() int64              { return int64(len(b)) }

func bytesReader(b []byte) io.Reader { return &sliceReader{b: b} }

type sliceReader struct{ b []byte }

func (s *sliceReader) Read(p []byte) (int, error) {
	if len(s.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.b)
	s.b = s.b[n:]
	return n, nil
}

// Output applies the edits of a tool to page-held text: the result is the
// input with every kept byte copied and every insertion added.
type Output struct {
	In  []byte
	Out []byte
	at  int64
}

func (o *Output) Keep(n int64) error {
	if o.at+n > int64(len(o.In)) {
		return errors.New("a text tool kept more text than it was given")
	}
	o.Out = append(o.Out, o.In[o.at:o.at+n]...)
	o.at += n
	return nil
}

func (o *Output) Drop(n int64) error {
	if o.at+n > int64(len(o.In)) {
		return errors.New("a text tool dropped more text than it was given")
	}
	o.at += n
	return nil
}

func (o *Output) Insert(p []byte) error { o.Out = append(o.Out, p...); return nil }

// Done checks that the tool accounted for all of its input.
func (o *Output) Done() error {
	if o.at != int64(len(o.In)) {
		return errors.New("a text tool did not account for all of its text")
	}
	return nil
}
