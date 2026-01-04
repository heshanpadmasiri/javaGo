class Parent {
  void foo() {
    System.out.println("foo")
  }

  void foo(int a) {
    System.out.println("foo with int")
  }

  void bar() {
    foo();
    foo(5)
  }

  class Child extends Parent {

    @override
    void foo() {
      System.out.println("child foo")
    }

    @override
    void foo(int a) {
      System.out.println("child foo with int")
    }

    void foo(String s) {
      System.out.println("child foo with string")
    }
  }
}
