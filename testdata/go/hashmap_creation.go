package converted

type mapExample struct {
}

func Test() {
	// migrated from hashmap_creation.java:2:5
	map1 := make(map[interface{}]interface{})
}

func newMapExample() mapExample {
	this := mapExample{}
	return this
}
