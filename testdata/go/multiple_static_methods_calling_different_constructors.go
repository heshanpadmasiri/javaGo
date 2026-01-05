package converted

type employee struct {
	name       string
	id         int
	department string
}

func CreateEngineer(name string, id int) Employee {
	// migrated from multiple_static_methods_calling_different_constructors.java:6:5
	return NewEmployeeFromStringIntString(name, id, "Engineering")
}

func CreateManager(name string, id int) Employee {
	// migrated from multiple_static_methods_calling_different_constructors.java:10:5
	return NewEmployeeFromStringIntString(name, id, "Management")
}

func NewEmployeeFromStringIntString(name string, id int, department string) employee {
	this := employee{}
	this.name = name
	this.id = id
	this.department = department
	return this
}

func NewEmployeeFromStringInt(name string, id int) employee {
	this := employee{}
	this.name = name
	this.id = id
	this.department = "Unknown"
	return this
}
