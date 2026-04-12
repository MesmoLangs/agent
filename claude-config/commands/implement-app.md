Implement a new Flutter feature end-to-end in the Mesmo app using BLoC + GetIt + GoRouter.

Procedure:
1. Data Models — features/{feature}/repository/{feature}_models.dart with fromJson
2. Repository (API Service) — CustomDio, POST requests, AppConfig.baseUrl, let errors propagate
3. BLoC Events — @Freezed event union in bloc/{feature}_event.dart
4. BLoC State — @Freezed state in bloc/{feature}_state.dart with @Default values
5. BLoC Handler — register handlers in constructor, emit(state.copyWith(...)), resolve repos via GetIt.I
6. DI Wiring — registerFactory for BLoC in bloc_module.dart, registerSingleton for service in service_module.dart
7. Screen & Widgets — BlocProvider + BlocBuilder, no logic in UI, dispatch events via context.read
8. Route Registration — add constant + GoRoute in app_router.dart
9. Context Extension — create context_extension.dart with reactive and non-reactive extensions
10. Run: `dart run build_runner build --delete-conflicting-outputs` then `flutter analyze`

$ARGUMENTS
