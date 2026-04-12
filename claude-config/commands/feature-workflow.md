Execute the full Mesmo development cycle: plan → implement server → implement app → review.

Steps:
0. Plan — investigate what exists, identify gaps, produce ordered implementation plan
1. Implement Server — GORM model → DTO → handler → route in main.go → constants. Run `go build ./...`
2. Implement App — data model → repository → BLoC events/state → BLoC handler → DI → screen → route. Run `dart run build_runner build --delete-conflicting-outputs` and `flutter analyze`
3. Review — architecture, DRY, security, version consistency. Fix all Critical/Major issues.

Important conventions:
- All work on `development` branch
- No automated tests unless explicitly asked
- Version bump after changes (all three values must match)
- Server before app for API dependencies
- No print/debugPrint/fmt.Println in production code
- No hardcoded values — use constants

$ARGUMENTS
