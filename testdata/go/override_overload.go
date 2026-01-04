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
	// migrated from override_overload.java:17:5
	System.out.println("child foo")
}

func (this *child) fooWithInt(a int) {
	// migrated from override_overload.java:22:5
	System.out.println("child foo with int")
}

func (this *child) fooWithString(s string) {
	// migrated from override_overload.java:27:5
	System.out.println("child foo with string")
}

func (this *parent) foo() {
	// migrated from override_overload.java:2:3
	System.out.println("foo")
}

func (this *parent) fooWithInt(a int) {
	// migrated from override_overload.java:6:3
	System.out.println("foo with int")
}

func (this *parent) bar() {
	// migrated from override_overload.java:10:3
	this.foo()
	// FIXME: more than one possible method for foo with 1 arguments

	this.fooWithInt(5)
}
