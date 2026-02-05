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
- the manager: it is in charge of managing all this components.
- the store: hold `Deployments` and `Generations` data structures.

### The builder

The builder actually evaluates and builds fetched commits. The builder
only runs a single task. It first evaluates a commit and then can
build it. If a new commit is submitted for evaluation, a current
running evaluation or build is stopped.

### The store

The store is in charge of managing all `Generations` and `Deployments`
used by comin. This is a centralized store, which is used by all
others components (the builder, deployer, ...) to get and update
`Generation` and `Deployment` objects.

The store is also in charge of the persisence: it writes all these
data to a file in order to preserve them across comin restarts.

## Design choices

### Protobuf

Protobuf has been introduced to be able to stream events from the
agent to the CLI client. This was pretty hard to achieve with the
previous HTTP Rest API.
