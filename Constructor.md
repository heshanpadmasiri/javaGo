# Adding proper support for constructors

- [ ] Replace new expression calls with calls to the proper constructor
  - [x] We need a utility method for getting the name for the constructor give type
    - We'll revisit this when we support overloading to pick the correct constructor
  - [ ] Then use that to call this method with the correct arguments
  - [ ] Add unit test for simple case

- [ ] Add default constructor (if none is present) that will take as argument all the fields

- [ ] Add constructor support for the abstract classes
