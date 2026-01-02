package converted

type Container interface {
}

type item struct {
	Name string
}

func newItem() Item {
	this := Item{}
	return this
}
