## Goal
The purpose of this test is to validate the robustness of kyverno's DeletingPolicy controller,
specifically to verify that:
-   Scheduled DeletingPolicy executions continue functioning even after a cleanup-controller restart
-   Deletion based on image conditions is re-evaluated correctly post-restart

## Scope of the test
This test verifies:
-   A DeletingPolicy with a cron-based schedule("*/1 * * * *")
-   A test pod with a container image(nginx)
-   Restart of the kyverno-cleanup-controller component
-   Continued policy enforcement after the restart
-   Automatic deletion of the targeted pod by the scheduled policy