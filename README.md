# javaGo

## Configuration

The migration tool can be configured using a `Config.toml` file in the current working directory.

### Config.toml Format

```toml
# Package name for generated Go code (optional, defaults to "main")
package_name = "mypackage"

# License header to prepend to generated files (optional)
license_header = """
// Copyright 2024 MyCompany
// Licensed under MIT
"""

# Type mappings from Java types to Go types (optional)
# Format: JavaTypeName = "go.package.path.GoTypeName"
[type_mappings]
DiagnosticCode = "diagnostics.DiagnosticCode"
SyntaxKind = "diagnostics.DiagnosticCode"
MyCustomType = "mypkg.CustomGoType"
```

### Type Mappings

Type mappings allow you to specify how custom Java types should be converted to Go types. This is useful when:

- You have Java types that should map to specific Go packages
- You want to use existing Go types instead of generating new ones
- You need to maintain compatibility with existing Go code

The type mapping takes precedence over the built-in type conversion rules. For example, if you define:

```toml
[type_mappings]
String = "mystring.CustomString"
```

Then all Java `String` types will be converted to `mystring.CustomString` instead of the built-in Go `string` type.

### Example Usage

Given a Java file:

```java
public class Diagnostic {
    private DiagnosticCode code;
    private String message;
}
```

With a `Config.toml`:

```toml
package_name = "diagnostics"

[type_mappings]
DiagnosticCode = "codes.DiagnosticCode"
```

The migration tool will generate:

```go
package diagnostics

type Diagnostic struct {
    code codes.DiagnosticCode
    message string
}
```

## Migration strategy

### Abstract classes

- Assume we have an abstract class called Foo.
- We generate the following Go artifacts:
  1. FooData: interface containing getters and setters for all fields
  2. FooBase: struct containing all the fields
  3. FooMethods: struct containing default (non-abstract) method implementations
  4. Foo: interface that embeds FooData

````java
abstract class Foo {
  int a;
  abstract int f();
  int b() {
    return f() + a;
  }
}
```go
// 1. Field access interface
type FooData interface {
  SetA(a int)
  GetA() int
}

// 2. Base with fields
type FooBase struct {
  A int
}

func (b *FooBase) SetA(a int) { b.A = a }
func (b *FooBase) GetA() int { return b.A }

// 3. Foo interface = fields + abstract + default methods
type Foo interface {
  FooData
  F() int // abstract
  B() int // default implementation in FooMethods
}

// 4. Default (non-abstract) methods
type FooMethods struct {
  Self Foo
}

func (m *FooMethods) B() int {
  return m.Self.F() + m.Self.GetA()
}
````

#### Subclass

```java
class Bar extends Foo {
  int f() {
    return 42
  }
}
```

```go
type Bar struct {
  FooBase
  FooMethods
}

func (b *Bar) F() int {
  return 42
}
```

### Overloading

- For overloading we need to generate different methods based on the parameter types.

```java
class Foo {
  int bar() {...}
  int bar(Baz baz) {...}
  int bar(Baz baz, BarBaz barBaz) {...}
  int bar(Baz baz, FooBaz fooBaz) {...}
}
```

```go
type Foo struct {}

func (this *foo) bar() {}
func (this *foo) barWithBaz(baz Baz) {}
func (this *foo) barWithBazBarBaz(baz Baz, barBaz BarBaz) {}
func (this *foo) barWithBazFooBaz(baz Baz, fooBaz FooBaz ) {}

```

- We can try to do a best case fix (since tool don't have type information) based on the number of arguments. If there are multiple such methods add a fix me comment

```java
int a = f.bar()
int b = f.bar(b)
int c = f.bar(b, bz)
int d = f.bar(b, fz)
```

```go
a := f.bar()
b := f.barWithBaz(b)
// FIXME: failed to distinguish between barWithBazBarBaz and barWithBazFooBaz
b := f.barWithBazBarBaz(b, bz)
// FIXME: failed to distinguish between barWithBazBarBaz and barWithBazFooBaz
b := f.barWithBazBarBaz(b, fz)

```

### Type casts

```java
Foo a = (Foo) b;
```

```go
a, ok := b.(Foo)
if !ok {
  panic("expected Foo")
}
```

### Enum

- Where possible try to use go enum with name prefixes to avoid clashes. Example in java if there is enum `Foo` with values `Bar` and `Baz` use

```go
type Foo uint
const (
  FOO_BAR Foo = iota
  FOO_BAZ Foo
)
```

- If the enum in question has data create a struct instead.

```go

type DiagnosticErrorCode struct {
	diagnosticId string
	messageKey   string
}

// Constants ported from io.ballerina.compiler.internal.diagnostics.DiagnosticErrorCode
// Generic syntax error
var ERROR_SYNTAX_ERROR = DiagnosticErrorCode{diagnosticId: "BCE0000", messageKey: "error.syntax.error"}
```

### Mutating parameters

In java code we will have cases where we mutate values passed in as parameters. _commonly we add to lists passed in_. To deal with this we are always passing lists/arrays as pointers to arrays in go code. /Currently there is no way to detect and properly migrated call sites/

