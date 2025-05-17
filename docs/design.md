## The comin commit selection algorithm

comin supports several remotes and each of these remotes can have a
`main` and `testing` branches. A new commit can be submitted in all of
these branches and comin need to decide which one to choose. 

The comin goal is to
- refuse commits push-forced to `main` branches
- only allow `testing` branches on top of `main` branches
- prefer commits from `testing` branches

Here is the algorithm used to choose the next commit to deploy:

1. Fetch a subset of remotes
2. Ensure commits from updated `main` and `testing` branches are not
   behind the last `main` deployed commit
3. Get the first commit from `main` branches (remotes are ordered in
   the configuration) on top of the last deployed `main` commit. If no
   such commit exists, comin gets the first commit equal to the last
   deployed `main` commit.
4. Get the first `testing` commit on top of the previously chosen
   `main` commit. If no such commit exists, comin uses the previously
   chosen `main` commit.

## Internal architecture

The main comin components are

- the fetcher: fetch commits from remote and produce `RepositoryStatus`.
- the builder: from a `RepositoryStatus`, it creates a `Generation` which is then evaluated and built.
- the deployer: from a `Generation`, it creates a `Deployment` which
  is used to decide how to run the `switch-to-configuration` script.
- the manager: it is in charge of managing all this components
