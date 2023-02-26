# Contributing to Ministream

This page contains information about reporting issues as well as some tips and
guidelines useful to experienced open source contributors. Finally, make sure
you read our [Code of conduct](#code-of-conduct) before you
start participating.


## Topics

- [Contributing to Ministream](#contributing-to-ministream)
	- [Topics](#topics)
	- [Contributing on source code](#contributing-on-source-code)
	- [Reporting security issues](#reporting-security-issues)
	- [Reporting other issues](#reporting-other-issues)
	- [Quick contribution tips and guidelines](#quick-contribution-tips-and-guidelines)
		- [Pull requests are always welcome](#pull-requests-are-always-welcome)
		- [Talking to other users and contributors](#talking-to-other-users-and-contributors)
		- [Conventions](#conventions)
		- [Merge approval](#merge-approval)
		- [Sign your work](#sign-your-work)
		- [How can I become a maintainer?](#how-can-i-become-a-maintainer)
	- [Code of conduct](#code-of-conduct)
	- [Coding Style](#coding-style)


## Contributing on source code

*Before opening a pull request*, review the [Contributing](CONTRIBUTING.md) page.

It lists steps that are required before creating a PR.

When you contribute code, you affirm that the contribution is your original work and that you
license the work to the project under the project's open source license. Whether or not you
state this explicitly, by submitting any copyrighted material via pull request, email, or
other means you agree to license the material under the project's open source license and
warrant that you have the legal authority to do so.


## Reporting security issues

The maintainers take security seriously. If you discover a security
issue, please bring it to their attention right away!

Please **DO NOT** file a public issue, instead, send your report privately by [Reporting a vulnerability](https://github.com/nbigot/ministream/security/advisories).


## Reporting other issues

A great way to contribute to the project is to send a detailed report when you
encounter an issue. We always appreciate a well-written, thorough bug report,
and will thank you for it!

Check that [our issue database](https://github.com/nbigot/ministream/issues)
doesn't already include that problem or suggestion before submitting an issue.
If you find a match, you can use the "subscribe" button to get notified of
updates. Do *not* leave random "+1" or "I have this too" comments, as they
only clutter the discussion, and don't help to resolve it. However, if you
have ways to reproduce the issue or have additional information that may help
resolve the issue, please leave a comment.

When reporting issues, always include:

* The output of `ministream -version`

If you are working on a specific git branch:

* The branch name `git rev-parse --abbrev-ref HEAD`
* The last git commit hash id `git log --format="%H" -n 1`

Also, include the steps required to reproduce the problem if possible and
applicable. This information will help us review and fix your issue faster.
When sending lengthy log files, consider posting them as a [gist](https://gist.github.com).
Don't forget to remove sensitive data from your log files before posting (you
can replace those parts with "REDACTED").


## Quick contribution tips and guidelines

This section gives the experienced contributor some tips and guidelines.

Please also read the [Github community guidelines](https://docs.github.com/en/site-policy/github-terms/github-community-guidelines).


### Pull requests are always welcome

Not sure if that typo is worth a pull request? Found a bug and know how to fix
it? Do it! We will appreciate it. Any significant change, like adding a backend,
should be documented as
[a GitHub issue](https://github.com/nbigot/ministream/issues)
before anybody starts working on it.

We are always thrilled to receive pull requests. We do our best to process them
quickly. If your pull request is not accepted on the first try,
don't get discouraged! If there's a problem with the implementation, hopefully you received feedback on what to improve.

Read the [Guide for collaborating with pullrequests](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests).


### Talking to other users and contributors

[Announcements](https://github.com/nbigot/ministream/discussions/categories/announcements)

[Forums / Discussions](https://github.com/nbigot/ministream/discussions)

[Questions and answers](https://github.com/nbigot/ministream/discussions/categories/q-a)


### Conventions

Fork the repository and make changes on your fork in a feature branch:

- If it's a bug fix branch, name it XXXX-something where XXXX is the number of
    the issue.
- If it's a feature branch, create an enhancement issue to announce
    your intentions, and name it XXXX-something where XXXX is the number of the
    issue.

Submit unit tests for your changes. Go has a great test framework built in; use
it! Take a look at existing tests for inspiration. Also, end-to-end tests are
available. Run the full test suite, both unit tests and e2e tests on your
branch before submitting a pull request. See [BUILDING.md](BUILDING.md) for
instructions to build and run tests.

Write clean code. Universally formatted code promotes ease of writing, reading,
and maintenance. Always run `gofmt -s -w file.go` on each changed file before
committing your changes. Most editors have plug-ins that do this automatically.

Pull request descriptions should be as clear as possible and include a reference
to all the issues that they address.

Commit messages must start with a capitalized and short summary (max. 50 chars)
written in the imperative, followed by an optional, more detailed explanatory
text which is separated from the summary by an empty line.

Code review comments may be added to your pull request. Discuss, then make the
suggested modifications and push additional commits to your feature branch. Post
a comment after pushing. New commits show up in the pull request automatically,
but the reviewers are notified only when you comment.

Pull requests must be cleanly rebased on top of the base branch without multiple branches
mixed into the PR.

**Git tip**: If your PR no longer merges cleanly, use `rebase main` in your
feature branch to update your pull request rather than `merge main`.

Before you make a pull request, squash your commits into logical units of work
using `git rebase -i` and `git push -f`. A logical unit of work is a consistent
set of patches that should be reviewed together: for example, upgrading the
version of a vendored dependency and taking advantage of its now available new
feature constitute two separate units of work. Implementing a new function and
calling it in another file constitute a single logical unit of work. The very
high majority of submissions should have a single commit, so if in doubt: squash
down to one.

After every commit, make sure the test suite passes. Include documentation
changes in the same pull request so that a revert would remove all traces of
the feature or fix.

Include an issue reference like `Closes #XXXX` or `Fixes #XXXX` in the pull
request description that closes an issue. Including references automatically
closes the issue on a merge.

Please see the [Coding Style](#coding-style) for further guidelines.


### Merge approval

Maintainers use LGTM (Looks Good To Me) in comments on the code review to
indicate acceptance.

A change requires at least 2 LGTMs from the maintainers of each
component affected.

For more details, see the [MAINTAINERS](MAINTAINERS.md) page.


### Sign your work

The sign-off is a simple line at the end of the explanation for the patch. Your
signature certifies that you wrote the patch or otherwise have the right to pass
it on as an open-source patch. The rules are pretty simple: if you can certify
the below (from [developercertificate.org](http://developercertificate.org/)):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
660 York Street, Suite 102,
San Francisco, CA 94110 USA

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

Then you just add a line to every git commit message:

    Signed-off-by: Joe Smith <joe.smith@email.com>

Use your real name (sorry, no pseudonyms or anonymous contributions.)

If you set your `user.name` and `user.email` git configs, you can sign your
commit automatically with `git commit -s`.


### How can I become a maintainer?

The procedures for adding new maintainers are explained in the global
[MAINTAINERS](MAINTAINERS.md)
file in the
[https://github.com/nbigot/ministream/](https://github.com/nbigot/ministream/)
repository.

Don't forget: being a maintainer is a time investment. Make sure you
will have time to make yourself available. You don't have to be a
maintainer to make a difference on the project!


## Code of conduct

Please read the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md).

Please also read the [Github Community Code of Conduct](https://docs.github.com/en/site-policy/github-terms/github-community-code-of-conduct).


## Coding Style

Unless explicitly stated, we follow all coding guidelines from the Go
community. While some of these standards may seem arbitrary, they somehow seem
to result in a solid, consistent codebase.

It is possible that the code base does not currently comply with these
guidelines. We are not looking for a massive PR that fixes this, since that
goes against the spirit of the guidelines. All new contributors should make their
best effort to clean up and make the code base better than they left it.
Obviously, apply your best judgement. Remember, the goal here is to make the
code base easier for humans to navigate and understand. Always keep that in
mind when nudging others to comply.

The rules:

1. All code should be formatted with `go fmt ./...`
2. All code should pass the rules of `go vet ./...`
3. All code should follow the guidelines covered in [Effective
   Go](http://golang.org/doc/effective_go.html) and [Go Code Review
   Comments](https://github.com/golang/go/wiki/CodeReviewComments).
4. Include code comments. Tell us the why, the history and the context.
5. Document _all_ declarations and methods, even private ones. Declare
   expectations, caveats and anything else that may be important. If a type
   gets exported, having the comments already there will ensure it's ready.
6. Variable name length should be proportional to its context and no longer.
   `noCommaALongVariableNameLikeThisIsNotMoreClearWhenASimpleCommentWouldDo`.
   In practice, short methods will have short variable names and globals will
   have longer names.
7. No underscores in package names. If you need a compound name, step back,
   and re-examine why you need a compound name. If you still think you need a
   compound name, lose the underscore.
8. No utils or helpers packages. If a function is not general enough to
   warrant its own package, it has not been written generally enough to be a
   part of a util package. Just leave it unexported and well-documented.
9. All tests should run with `go test ./...` and outside tooling should not be
   required. No, we don't need another unit testing framework. Assertion
   packages are acceptable if they provide _real_ incremental value.
10. Even though we call these "rules" above, they are actually just
    guidelines. Since you've read all the rules, you now know that.

If you are having trouble getting into the mood of idiomatic Go, we recommend
reading through [Effective Go](https://golang.org/doc/effective_go.html). The
[Go Blog](https://blog.golang.org) is also a great resource. Drinking the
kool-aid is a lot easier than going thirsty.
