class Employee {
    private String name;
    private int id;
    private String department;
    
    public static Employee createEngineer(String name, int id) {
        return new Employee(name, id, "Engineering");
    }
    
    public static Employee createManager(String name, int id) {
        return new Employee(name, id, "Management");
    }
    
    public Employee(String name, int id, String department) {
        this.name = name;
        this.id = id;
        this.department = department;
    }
    
    public Employee(String name, int id) {
        this.name = name;
        this.id = id;
        this.department = "Unknown";
    }
}
