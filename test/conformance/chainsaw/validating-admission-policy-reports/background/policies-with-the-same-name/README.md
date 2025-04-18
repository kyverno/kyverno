## Description

This test verifies that a report is generated successfully when multiple policies with the same name (ValidatingPolicy, ImageValidatingPolicy, and ValidatingAdmissionPolicy with its binding) are applied to a single resource.

## Steps

1. Create a deployment that has the `env` label set to `prod` with an unsigned image.

2. Create a ValidatingPolicy that checks for the `env` label to be set to `prod`.

3. Create an ImageValidatingPolicy that checks for the image to be signed.

4. Create a ValidatingAdmissionPolicy that checks for the `env` label to be set to `prod`.

5. It is expected that a policy report is generated for the deployment with three results:
    - PASS result for the ValidatingPolicy
    - FAIL result for the ImageValidatingPolicy
    - PASS result for the ValidatingAdmissionPolicy
