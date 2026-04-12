# Project Instructions

## General Rules

Never put any comments in the code. Use descriptive variable or function names instead.
Never leave any console.log, console.error, console.info, or console.warn statements in the code.
Never use hardcoded values. Always use enums and constants. Before defining a new constant, search the codebase for existing ones that match.

## Mesmo Go Server (server/)

Stack: Go + Fiber + GORM + PostgreSQL

### Structure
```
server/
├── model/database/       → GORM database models (entities)
├── model/handlers/       → Request/response DTOs (plain structs)
├── handlers/{feature}/   → Handler files with HTTP logic
├── handlers/common/      → Shared: logger, helpers, HandleRequest wrapper
├── config/config.go      → ServerVersion, RequiredAppVersion, constants
├── config/constants.go   → Domain constants, enums (as const strings)
└── main.go               → Route registrations (all routes live here)
```

### Handler Pattern
Every handler uses the `common.HandleRequest[Req, Resp]()` generic wrapper — never write raw Fiber response handling.
Each handler folder has one public function per endpoint, delegating to a private implementation.
Loggers are initialized at file level: `var debug, errorz, withError, info, _ = logger.GetLoggers("featureName")`
Never hardcode values — add them to `config/constants.go`.
No `fmt.Println`, `log.Println` — use the zerolog logger exclusively.
GORM models live in `model/database/`, DTOs live in `model/handlers/`.
Routes are only registered in `main.go`.

### Handler Template
```go
package feature

import (
    "server-go/handlers/common"
    "server-go/handlers/common/logger"
    "server-go/model"
    "github.com/gofiber/fiber/v2"
)

var debug, errorz, withError, info, _ = logger.GetLoggers("feature")

func HandleActionEntity(fiber *fiber.Ctx) error {
    return common.HandleRequest(
        common.HandleRequestParams[model.ActionEntityReq, model.Entity]{
            Fiber:           fiber,
            Handler:         handleActionEntity,
            ValidBodyParams: true,
        })
}

func handleActionEntity(body model.ActionEntityReq) (model.Entity, error) {
    info().Msgf("handleActionEntity userId=%s", body.UserID)
    return result, nil
}
```

### Route Naming Convention
`/feature/action.entity` (noun + verb with dot separator)

### DB Access
```go
import "../../../agent/claude-config/server-go/database"
database.DB.Where("user_id = ?", userID).Find(&items)
database.DB.Where("id = ? AND user_id = ?", id, userID).First(&item)
database.DB.Create(&item)
database.DB.Save(&item)
database.DB.Delete(&item, "id = ? AND user_id = ?", id, userID)
```

## Mesmo Flutter App (app/)

Stack: Flutter + BLoC + GetIt + GoRouter + Freezed + ObjectBox

### Structure
```
app/lib/
├── features/{feature}/
│   ├── bloc/                 # BLoC events, states, handler logic
│   ├── repository/           # API calls (Dio-based) + data models
│   └── ui/                   # Screens and widgets
├── features/shared/          # Reusable widgets, constants, utils
├── injector/                 # GetIt DI modules (service, repository, bloc)
├── router/                   # GoRouter config + route constants
└── app_config.dart           # appVersion, baseUrl, feature flags
```

### BLoC Rules
- All business logic in BLoC, views only dispatch events + render state
- Events/states use `@Freezed()` with `part"../../../agent/claude-config"` directives
- Dispatch: `context.featureBloc.add(Event())` or `context.read<Bloc>().add(Event())`
- Catch errors in handlers, emit error state — don't throw unhandled
- Handler signature: `FutureOr<void> _handler(Event event, Emitter<State> emit)`
- Repositories resolved via `GetIt.I<RepositoryType>()` at file top as final variable
- BLoCs always use `registerFactory` (new instance per use); services use `registerSingleton`

### Theme & Localization
Never hardcode colors — use `context.appTheme` (primary, onPrimary, secondary, divider, icon, error, etc.)
Never hardcode user-facing strings — use `S.current.Key` from `lib/generated/l10n.dart`

### Context Extensions (Preferred)
Each feature has `context_extension.dart`:
```dart
extension FeatureX on BuildContext {
  FeatureState get featureState => select((FeatureBloc b) => b.state); // reactive
}
extension FeatureReaderX on BuildContext {
  FeatureBloc get featureBloc => read<FeatureBloc>(); // non-reactive
}
```

### Repository Pattern
Uses `CustomDio` injected via GetIt. All requests are POST. Let errors propagate to BLoC.
```dart
final baseUrl = '${AppConfig.baseUrl}/feature';
class FeatureRepository {
  CustomDio dio;
  FeatureRepository({required this.dio});
  Future<Item> create({required String name}) async {
    final response = await dio.post('$baseUrl/create', data: {"name": name});
    return Item.fromJson(response.data);
  }
}
```

### Inter-BLoC Communication
Use `eventStream` (pub/sub), NOT BlocListener chains.

### After Freezed Changes
```bash
dart run build_runner build --delete-conflicting-outputs
```

## Version Consistency Rule

All three version values must always match:

| File | Field |
|------|-------|
| `app/lib/app_config.dart` | `AppConfig.appVersion` |
| `server/config/config.go` | `ServerVersion` |
| `server/config/config.go` | `RequiredAppVersion` |

## Commit Message Format

Use Conventional Commits: `<type>(<scope>): <description>`
Types: feat, fix, refactor, perf, style, test, docs, build, ops, chore
Description: imperative, present tense, no capital first letter, no trailing dot

## Branch Strategy

All work happens on the `development` branch. Never commit to `main` directly.

## Git Workflow — YOU MUST DO THIS

You are running inside a Docker container with full git access. The Telegram bot that calls you does NOTHING except relay messages — YOU are responsible for the entire workflow:

1. Make code changes
2. Run `git add -A`
3. Commit with conventional commit message: `git commit -m "feat(scope): description"`
4. Push to development: `git push origin development`
5. Bump version (patch for fixes, minor for features):
   - If app changed: update `AppConfig.appVersion` in `app/lib/app_config.dart`
   - If server changed: update `ServerVersion` and `RequiredAppVersion` in `server/config/config.go`
   - All three values must always match
6. Commit the version bump: `git commit -am "chore: bump version to X.Y.Z"`
7. Push again: `git push origin development`
8. Create and push tag:
   - App changes: `git tag ios-vX.Y.Z && git push origin ios-vX.Y.Z`
   - Server changes: `git tag qa-vX.Y.Z && git push origin qa-vX.Y.Z`
   - NEVER create `prod-v*` tags

If you skip any of these steps, the deployment pipeline will not trigger. Always complete the full workflow.
