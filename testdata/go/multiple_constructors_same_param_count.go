package converted

type container struct {
	value interface{}
}

func NewContainerFromString(s string) container {
	this := container{}
	this.value = s
	return this
}

func NewContainerFromInt(i int) container {
	this := container{}
	this.value = i
	return this
}

func Test() {
	// migrated from multiple_constructors_same_param_count.java:12:5
	// FIXME: more than one possible constructor for Container

	c := NewContainerFromString("test")
}
