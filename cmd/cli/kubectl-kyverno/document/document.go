package document

type Document interface {
	Content() ([]byte, error)
}
