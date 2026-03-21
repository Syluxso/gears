# Route Organization Pattern

**Documented:** 2026-03-20
**Status:** Reference Example

## Overview

A structured approach to organizing application routes using prefixes, middleware groups, and naming conventions. Separates concerns between different user types (admin, client, public) while maintaining clean URLs and consistent access control.

**Location**: `routes/web.php` or equivalent routing file
**Pattern**: Prefix-based route organization with middleware groups

## Key Features

### 1. Route Prefixes for User Realms

Different prefixes separate major user types and their distinct feature sets.

#### Implementation

```php
// Admin routes - /admin/*
Route::prefix('admin')->group(function () {
    Route::middleware('guest')->group(function () {
        Route::get('/login', [AdminAuthController::class, 'showLogin'])->name('admin.login');
        Route::post('/login', [AdminAuthController::class, 'login']);
    });

    Route::middleware(['auth', 'admin'])->group(function () {
        Route::get('/dashboard', [AdminDashboardController::class, 'index'])->name('admin.dashboard');
        Route::resource('users', AdminUserController::class)->names('admin.users');
    });
});

// Client routes - /app/*
Route::prefix('app')->middleware(['auth', 'client'])->group(function () {
    Route::get('/dashboard', [ClientDashboardController::class, 'index'])->name('client.dashboard');
    Route::resource('projects', ProjectController::class)->names('client.projects');
});

// Public routes - /
Route::get('/', [HomeController::class, 'index'])->name('home');
Route::get('/about', [PageController::class, 'about'])->name('about');
```

### 2. Named Routes with Namespace Convention

Route names follow a consistent pattern based on their prefix and purpose.

```php
// Admin routes use 'admin.' prefix
->name('admin.dashboard')
->names('admin.users')  // generates admin.users.index, admin.users.store, etc.

// Client routes use 'client.' prefix
->name('client.dashboard')
->names('client.projects')

// Public routes use no prefix
->name('home')
->name('about')
```

### 3. Middleware Layering

Middleware is applied at different levels based on scope.

## Usage Patterns

Routes are accessed via named route helpers, making it easy to change URLs without breaking links throughout the application.

#### Example Usage

```php
// In views/controllers
redirect()->route('admin.dashboard');
route('client.projects.show', $project);
<a href="{{ route('admin.users.edit', $user) }}">Edit User</a>

// Checking current route
request()->routeIs('admin.*')  // true for any admin route
request()->routeIs('client.projects.index')  // exact match
```

## Route Structure by Realm

**Admin Realm** (`/admin/*`):

- `GET /admin/login` → Admin login form
- `GET /admin/dashboard` → Admin dashboard
- `GET /admin/users` → User management list
- `GET /admin/users/{id}/edit` → Edit user form
- `PUT /admin/users/{id}` → Update user

**Client Realm** (`/app/*`):

- `GET /app/dashboard` → Client dashboard
- `GET /app/projects` → Project list
- `POST /app/projects` → Create project
- `GET /app/projects/{id}` → View project

**Public Realm** (`/`):

- `GET /` → Homepage
- `GET /about` → About page
- `POST /contact` → Contact form submission

## Key Decisions

**Why Prefix-Based Organization?**

- Clear separation between admin, client, and public areas
- Easier to apply middleware to entire sections
- URL structure reflects permission boundaries
- Simpler to understand which routes belong to which user type

**Why Named Routes?**

- Change URLs without updating every reference
- IDE autocomplete for route names
- Easier to maintain when routes change
- Self-documenting (name indicates purpose)

## References

- Framework routing documentation
- RESTful resource controller conventions
