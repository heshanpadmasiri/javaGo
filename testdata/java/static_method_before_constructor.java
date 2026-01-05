class Person {
    private String name;
    private int age;
    
    public static Person createDefault() {
        return new Person("Unknown", 0);
    }
    
    public Person(String name, int age) {
        this.name = name;
        this.age = age;
    }
}
