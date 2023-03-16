# Title

Checks that more complex image extraction with keyless verification and required=true is working by submitting a Task which uses a verified container image. The Task should be created and the annotation `kyverno.io/verify-images` written which contains the image with digest and `true` indicating it was verified.