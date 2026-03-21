# Service Layer Pattern

**Documented:** 2026-03-20
**Status:** Reference Example

## Overview

A service layer architecture that encapsulates business logic separate from controllers and models. Services handle complex operations, coordinate between multiple models, and provide a clean API for controllers.

**Location**: `app/Services/` (or equivalent for your framework)
**Pattern**: Single-responsibility service objects with dependency injection

## Key Features

### 1. Business Logic Encapsulation

Services contain all business rules and operations, keeping controllers thin and focused on HTTP concerns.

#### Implementation

```php
class UserRegistrationService
{
    public function __construct(
        private UserRepository $users,
        private EmailService $emailer,
        private EventDispatcher $events
    ) {}

    public function register(array $data): User
    {
        // Validate business rules
        $this->validateRegistrationRules($data);

        // Create user
        $user = $this->users->create([
            'name' => $data['name'],
            'email' => $data['email'],
            'password' => Hash::make($data['password']),
        ]);

        // Send welcome email
        $this->emailer->sendWelcome($user);

        // Dispatch event
        $this->events->dispatch(new UserRegistered($user));

        return $user;
    }
}
```

### 2. Dependency Injection

Services receive their dependencies through constructor injection, making them testable and loosely coupled.

```php
// In controller
class AuthController extends Controller
{
    public function __construct(
        private UserRegistrationService $registrationService
    ) {}

    public function register(Request $request)
    {
        $user = $this->registrationService->register(
            $request->validated()
        );

        return redirect()->route('dashboard');
    }
}
```

### 3. Reusability

Services can be used across multiple controllers, console commands, jobs, and other services.

## Usage Patterns

Services are instantiated automatically by the framework's dependency injection container. Controllers, commands, and other services simply type-hint the service they need.

#### Example Usage

```php
// In controller
public function __construct(private OrderProcessingService $orderService) {}

// In console command
public function handle(OrderProcessingService $orderService) {
    $orderService->processDaily();
}

// In job
public function handle() {
    app(OrderProcessingService::class)->process($this->orderId);
}
```

## Key Decisions

**Why Service Layer?**

- Controllers became bloated with complex logic
- Same logic needed in web requests, API calls, and console commands
- Testing was difficult when logic was embedded in controllers
- Domain logic mixed with HTTP concerns

**Service vs. Action Pattern**: Chose services over single-action classes because most business operations require multiple coordinated steps and shared dependencies.

## References

- Martin Fowler's Service Layer pattern
- Framework documentation on dependency injection
