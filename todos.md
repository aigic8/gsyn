# todos

- [ ] use `protobuf` instead of `json` in API
- [x] extract api utility functions to seperate package
- [ ] add helper functions to client with generics
- [x] support remote destinations
- [x] normalize local paths (make them absolute)
- [ ] better logging
- [x] test cmd (dPath methods)
- [ ] use path validator in api server
- [ ] find a way to resolve https MITM attacks with no domains
- [x] check if a path is a pattern or just a file and error out if it is a file and doesnt match anything
- [ ] add support for recursive copies (src to be a folder)
- [ ] find a way to test remote copying 
- [x] remove panics from cmd/copy (better error handling)
- [ ] !IMPORTANT! we are not closing readers for dynamic paths right now, which I do think is dangerous.