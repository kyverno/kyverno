# Contributing to Kyverno

We welcome all contributions, suggestions, and feedback, so please do not hesitate to reach out!


## Ways you can contribute:
   - [Report Issues](https://github.com/kyverno/kyverno/blob/main/CONTRIBUTING.md#report-issues)
   - [Submit Pull Requests](https://github.com/kyverno/kyverno/blob/main/CONTRIBUTING.md#submit-pull-requests)
   - [Fix or Improve Documentation](https://github.com/kyverno/kyverno/blob/main/CONTRIBUTING.md#fix-or-improve-documentation) 
   - [Join Our Community Meetings](https://github.com/kyverno/kyverno/blob/main/CONTRIBUTING.md#join-our-community-meetings) 

### Report issues
   - Report potential bugs
   - Request a feature
   - Request a sample policy

### Submit Pull Requests
#### Setup local development environments 
-  Please refer to [Running in development mode](https://github.com/kyverno/kyverno/wiki/Running-in-development-mode) for local setup.

####  Submit a PR for [open issues](https://github.com/kyverno/kyverno/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
### Fix or Improve Documentation
   - [Kyverno Docs](https://github.com/kyverno/website)
   #### Get started

Head over to project repository on github and click the **"Fork"** button. With the forked copy, you can try new ideas and implement changes to the project.

 -  **Clone the repository to your device:**

Get the link of your forked repository, paste it in your device terminal and clone it using the command.

```
$ git clone https://hostname/YOUR-USERNAME/YOUR-REPOSITORY
```

 - **Create a branch:** 

 Create a new brach and navigate to the branch using this command.

 ```
 $ git checkout -b <new-branch>
 ```

 Great, its time to start hacking, You can now go ahead to make all the changes you want.


 - **Stage, Commit and Push changes:**

 Now that we have implemented the required changes, use the command below to stage the changes and commit them

 ```
 $ git add .
 ```

 ```
 $ git commit -s -m "Commit message"
 ```

 The -s signifies that you have signed off the the commit.

 Go ahead and push your changes to github using this command.
 
 ``` 
 $ git push 
 ```




Before you contribute, please review and agree to abide by our community [Code of Conduct](/CODE_OF_CONDUCT.md).
### Join Our Community Meetings
 The easiest way to reach us is on the [Kubernetes slack #kyverno channel](https://app.slack.com/client/T09NY5SBT/CLGR9BJU9). 
## Developer Certificate of Origin (DCO) Sign off

For contributors to certify that they wrote or otherwise have the right to submit the code they are contributing to the project, we are requiring everyone to acknowledge this by signing their work.

To sign your work, just add a line like this at the end of your commit message:

```
Signed-off-by: Random J Developer <random@developer.example.org>
```

This can easily be done with the `-s` command line option to append this automatically to your commit message.
```
$ git commit -s -m 'This is my commit message'
```

By doing this you state that you can certify the following (https://developercertificate.org/):
```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
1 Letterman Drive
Suite D4700
San Francisco, CA, 94129

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```