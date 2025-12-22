public interface Printable {
    void print();
}

public record Person(String name, int age) implements Printable {
    public void print() {
        System.out.println("Person: " + name + ", Age: " + age);
    }
}

