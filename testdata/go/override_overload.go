package converted

type child struct {
	Parent
}

type parent struct {
}

func newChild() child {
	this := child{}
	return this
}

func newParent() parent {
	this := parent{}
	return this
}

func (this *child) foo() {
	System.out.println("child foo")
}

func (this *child) fooWithInt(a int) {
	System.out.println("child foo with int")
}

func (this *child) fooWithString(s string) {
	System.out.println("child foo with string")
}

func (this *parent) foo() {
	System.out.println("foo")
}

func (this *parent) fooWithInt(a int) {
	System.out.println("foo with int")
}

func (this *parent) bar() {
	this.foo()
	// FIXME: more than one possible method for foo with 1 arguments

	this.fooWithInt(5)
}
