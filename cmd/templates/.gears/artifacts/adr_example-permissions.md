# Role-Based Permission System

**Documented:** 2026-03-20
**Status:** Real Implementation Example

## Overview

A comprehensive role-based access control (RBAC) system using Spatie's Laravel Permission package. Provides flexible permission management with support for both direct user permissions and role-based permissions.

**Location**: User model + middleware + database tables  
**Package**: `spatie/laravel-permission` v7.x  
**Configuration**: `config/permission.php`

## Key Features

### 1. User Model Integration

Users gain role and permission capabilities through a trait added to the model.

#### Implementation

```php
use Spatie\Permission\Traits\HasRoles;

class User extends Authenticatable
{
    use HasFactory, Notifiable, HasRoles;

    // Model automatically gets these methods:
    // - assignRole(), removeRole(), hasRole()
    // - givePermissionTo(), revokePermissionTo(), hasPermissionTo()
    // - getAllPermissions(), getPermissionNames()
}
```

### 2. Database Structure

Five tables manage the permission system:

**Tables**:

- `permissions` - Individual permission definitions
  - `id` - Primary key
  - `name` - Permission name (e.g., `users.view`, `invoices.create`)
  - `guard_name` - Auth guard (typically `web`)
  - `created_at`, `updated_at`

- `roles` - Role definitions
  - `id` - Primary key
  - `name` - Role name (e.g., `admin`, `manager`, `viewer`)
  - `guard_name` - Auth guard
  - `created_at`, `updated_at`

- `model_has_permissions` - Direct permission assignments to users
  - `permission_id`, `model_type`, `model_id`
  - Polymorphic relationship allows any model to have permissions

- `model_has_roles` - Role assignments to users
  - `role_id`, `model_type`, `model_id`
  - Polymorphic relationship

- `role_has_permissions` - Permissions assigned to roles
  - `permission_id`, `role_id`
  - Defines which permissions each role grants

### 3. Middleware Protection

Routes are protected using middleware that checks roles/permissions before allowing access.

```php
// In bootstrap/app.php
$middleware->alias([
    'role' => \Spatie\Permission\Middleware\RoleMiddleware::class,
    'permission' => \Spatie\Permission\Middleware\PermissionMiddleware::class,
    'role_or_permission' => \Spatie\Permission\Middleware\RoleOrPermissionMiddleware::class,
]);

// In routes
Route::middleware(['auth', 'role:admin'])->group(function () {
    Route::get('/admin/users', [UserController::class, 'index']);
});

Route::middleware(['auth', 'permission:users.create'])->group(function () {
    Route::post('/admin/users', [UserController::class, 'store']);
});
```

## Permission Naming Convention

Pattern: `{resource}.{action}`

**Standard Actions**:

- `view_any` - List/index (view collection)
- `view` - Show single resource
- `create` - Create new resource
- `update` - Modify existing resource
- `delete` - Remove/archive resource

**Custom Actions** (as needed):

- `approve`, `publish`, `assign`, etc.

**Examples**:

- `users.view_any` - Can list users
- `users.create` - Can create new users
- `invoices.update` - Can edit invoices
- `tenants.assign_manager` - Can assign managers to tenants

## Usage Patterns

#### Assigning Roles

```php
// Give user a role
$user->assignRole('admin');
$user->assignRole(['manager', 'editor']);

// Remove role
$user->removeRole('editor');

// Check if user has role
if ($user->hasRole('admin')) {
    // User is admin
}
```

#### Granting Direct Permissions

```php
// Give permission directly to user
$user->givePermissionTo('users.delete');

// Revoke permission
$user->revokePermissionTo('users.delete');

// Check permission
if ($user->hasPermissionTo('users.create')) {
    // User can create users
}
```

#### Assigning Permissions to Roles

```php
$role = Role::findByName('manager');

// Give permissions to role
$role->givePermissionTo(['users.view', 'users.create', 'users.update']);

// All users with 'manager' role now have these permissions
```

#### In Blade Views

```blade
@role('admin')
    <a href="/admin/settings">Settings</a>
@endrole

@can('users.create')
    <button>Add User</button>
@endcan

@hasanyrole('admin|manager')
    <div>Management Panel</div>
@endhasanyrole
```

#### In Controllers

```php
public function store(Request $request)
{
    // Manual authorization check
    if (! $request->user()->can('users.create')) {
        abort(403);
    }

    // Or use authorize helper
    $this->authorize('create', User::class);

    // Create user...
}
```

## Super User Implementation

A common pattern is to bypass all permission checks for super admins:

```php
// In AppServiceProvider::boot()
Gate::before(function ($user, string $ability): ?bool {
    if ($user->hasRole('super_admin')) {
        return true; // Bypass all permission checks
    }

    return null; // Continue normal authorization
});
```

This allows super admins to perform any action without explicitly assigning every permission.

## Seeding Roles and Permissions

```php
// In RolesAndPermissionsSeeder
use Spatie\Permission\Models\Role;
use Spatie\Permission\Models\Permission;

public function run()
{
    // Reset cached roles and permissions
    app()[\Spatie\Permission\PermissionRegistrar::class]->forgetCachedPermissions();

    // Create permissions
    Permission::create(['name' => 'users.view_any']);
    Permission::create(['name' => 'users.create']);
    Permission::create(['name' => 'users.update']);
    Permission::create(['name' => 'users.delete']);

    // Create roles and assign permissions
    $adminRole = Role::create(['name' => 'admin']);
    $adminRole->givePermissionTo(Permission::all());

    $managerRole = Role::create(['name' => 'manager']);
    $managerRole->givePermissionTo(['users.view_any', 'users.view', 'users.update']);

    $viewerRole = Role::create(['name' => 'viewer']);
    $viewerRole->givePermissionTo(['users.view_any', 'users.view']);
}
```

## Key Decisions

**Why Spatie Laravel Permission?**

- Industry standard, well-maintained package
- Flexible: supports direct permissions AND role-based permissions
- Database-driven (can modify permissions without code changes)
- Built-in Blade directives and middleware
- Caches permissions for performance

**Resource.Action Naming**: Makes it easy to see what resource a permission affects and what action it allows. Groups logically in UI.

**Super Admin Gate Bypass**: Prevents need to assign hundreds of permissions to super admins. They automatically get access to everything.

## References

- Spatie Laravel Permission Docs: https://spatie.be/docs/laravel-permission
- Package: `composer require spatie/laravel-permission`
- Migration: `php artisan vendor:publish --provider="Spatie\Permission\PermissionServiceProvider"`
