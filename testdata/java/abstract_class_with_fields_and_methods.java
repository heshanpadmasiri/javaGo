abstract class Foo {
    int a;
    abstract int f();
    int b() {
        return f() + a;
    }
}