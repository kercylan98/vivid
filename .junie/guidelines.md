# Vivid Project Guidelines for Junie

## Project Overview
Vivid is a Go-based implementation of the Actor model for distributed systems. It provides a powerful, flexible, and high-performance framework for building scalable concurrent and distributed systems. The Actor model simplifies concurrent programming by breaking systems down into independent, encapsulated computational units (Actors) that communicate through message passing.

## Project Structure
- `/src/vivid/` - Main package containing the public API
  - Core types: Actor, ActorSystem, ActorContext, ActorRef
  - Configuration types: ActorConfig, ActorSystemConfig
  - Message types and supervision strategies
- `/src/vivid/internal/` - Internal implementation details
  - `/core/` - Core implementations of the Actor model
  - `/actx/` - Actor context implementations
  - `/queues/` - Queue implementations for message passing
  - `/mailbox/` - Mailbox implementations for Actors
  - `/ref/` - Actor reference implementations
  - `/future/` - Future pattern implementations

## Testing Guidelines
- Run tests using standard Go testing commands:
  ```
  go test ./src/vivid/...
  ```
- Tests are written using the standard Go testing package
- When implementing changes, ensure all existing tests pass
- For new features, add appropriate test coverage
- Test both normal operation and error handling scenarios

## Build Requirements
- Go 1.24.0 or higher is required
- Build the project using standard Go commands:
  ```
  go build ./src/vivid/...
  ```
- No special build flags are required

## Code Style Guidelines
- Follow standard Go code style conventions
- Use meaningful variable and function names
- Add appropriate comments for public APIs
- All comments and documentation must be written in Chinese
- Keep functions focused and concise
- Use error handling patterns consistent with the rest of the codebase
- Maintain backward compatibility for public APIs when possible

## Contribution Workflow
When working on this project, Junie should:
1. Understand the issue or feature request thoroughly
2. Run tests to ensure the current codebase is working correctly
3. Make minimal changes to address the issue
4. Ensure all tests pass after changes
5. Verify that the changes maintain backward compatibility
6. Submit the solution with a clear explanation of the changes made

## Documentation
- Public APIs should be well-documented with comments
- All documentation and comments must be written in Chinese
- Follow Go's documentation conventions
- Update README.md if necessary when adding significant features
