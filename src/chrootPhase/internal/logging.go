package internal

// Vector logging is entirely package-shipped (ADR-013):
//   - vector-config carries the configuration, the log sinks (its
//     post-install) and the runtime paths the config names — a tmpfiles
//     fragment for dinit and the two-command /etc/init.d/vector-setup for
//     OpenRC (which has no tmpfiles processor);
//   - vector-dinit carries the service and only its own runtime state.
// The installer writes nothing for it.
