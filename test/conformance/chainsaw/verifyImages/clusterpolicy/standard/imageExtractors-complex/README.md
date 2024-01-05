# Title

Checks that more complex image extraction is working by submitting a Task which uses an unverified container image. The Task should fail to be created since the supplied public key is not valid for it (the image is unsigned).