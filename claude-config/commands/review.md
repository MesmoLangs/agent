Perform a structured code review for the Mesmo Go server and Flutter app. Surface issues with severity levels.

Severity: Critical > Major > Minor > Nit

Sections to review:
1. Architecture & Layer Correctness
   - Server: business logic in private handler, HandleRequest wrapper, routes only in main.go, file-level loggers
   - App: logic in BLoC only, API calls through repository only, GetIt for DI, AppRouter constants for navigation
2. Code Quality & DRY
   - Duplicated patterns, naming consistency, magic strings/numbers, dead code, error handling
   - Go: withError(err).Msgf for errors, all GORM tx.Error checked
   - Flutter: freezed files regenerated, copyWith in both success and error paths
3. Security
   - Server: user identity validated, GORM parameterized queries, no sensitive field leaks
   - App: no tokens logged, input validated before API calls
4. Version Consistency — all three version values must match:
   - app/lib/app_config.dart AppConfig.appVersion
   - server/config/config.go ServerVersion
   - server/config/config.go RequiredAppVersion
5. Release Readiness — no print/debugPrint/fmt.Println, no hardcoded localhost URLs

For each issue: number it, assign severity, describe with file references, suggest fix.

$ARGUMENTS
