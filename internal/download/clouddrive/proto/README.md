# CloudDrive2 protocol subset

`clouddrive.proto` is the minimal wire-compatible subset of the official
CloudDrive2 v1.0.14 protocol used by JavBoss. Keeping the subset local avoids
coupling the application to unrelated CloudDrive2 RPCs while retaining their
official field numbers.

Regenerate the Go bindings after changing the proto:

```sh
protoc -I . \
  --go_out=paths=source_relative:. \
  --go-grpc_out=paths=source_relative:. \
  clouddrive.proto
```

Compare changes against <https://www.clouddrive2.com/api/clouddrive.proto>
before updating the version.
