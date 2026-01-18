package converted

type test struct {
	value int
	name  string
}

var INSTANCE = NewTestFromIntString(42, "example")

// FIXME: more than one possible constructor for Test
var AMBIGUOUS = NewTestFromIntIntInt(0, 0, 0)

func NewTestFromIntString(value int, name string) test {
	this := test{}
	this.value = value
	this.name = name
	return this
}

func NewTestFromIntIntInt(a int, b int, c int) test {
	this := test{}
	this.value = a
	return this
}

func NewTestFromIntStringInt(x int, y string, z int) test {
	this := test{}
	this.value = x
	this.name = y
	return this
}
