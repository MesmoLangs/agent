Investigate what exists in both the Go server and Flutter app for the relevant domain. Identify gaps, determine what needs to be built on each side, and define the implementation order.

Procedure:
1. Understand the Goal — which domain, does it need new endpoints, what data shapes
2. Investigate What Exists:
   - Server: handlers/{feature}/, model/database/, model/handlers/, main.go routes, config/constants.go
   - App: features/{feature}/ BLoC, repository, UI, injector/ wiring, router/app_router.dart
3. Identify the Gap — missing handlers, models, DTOs, BLoC events, repository methods, screens
4. Produce the Plan:
   - Summary
   - Server Changes (model → DTO → handler → route → constants)
   - App Changes (BLoC event → state → handler → repository → UI → route → DI)
   - Order of Implementation (server first, then app)
   - Version / Release Notes
5. Validate — every endpoint app needs exists, GORM fields present, routes registered

$ARGUMENTS
