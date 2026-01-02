class Person {
    private String name;
    private int age;
    
    public Person(String name, int age) {
        this.name = name;
        this.age = age;
    }
    
    public static void test() {
        Person p = new Person("Alice", 30);
    }
}
