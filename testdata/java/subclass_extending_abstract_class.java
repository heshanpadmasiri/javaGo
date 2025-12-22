abstract class Foo {
    int a;
    abstract int f();
    int b() {
        return f() + a;
    }
}
class Bar extends Foo {
    int f() {
        return 42;
    }
}