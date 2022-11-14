## How to test a configuration

1. Create a testing branch in your configuration repository.
2. Push this branch

- The testing branch have to have the HEAD of master as ancestor
- To promote the change, we have to rebase the master branch on top of
  the testing branch
- It is possible to push force the testing branch, but it always have
  to have the current HEAD of master as ancestor

If new valid commits are pushed to the master branch, comin switches
to it and deploy it.

To stop using the testing branch, reset it to the main branch HEAD.
