## The comin commit selection algorithm

The comin configuration can contains several remotes and each of these
remotes can have a Main and Testing branches. We then need an
algorithm to determine which commit we want to deploy.

1. Fetch `Main` and `Testing` branches from poll remotes
2. Ensure commits from these branches are not behind the last `Main` commit
3. Get the first commit from `Main` branches (remotes are ordered in
   the configuration) strictly on top of the reference Main Commit. If
   not found, get the first commit equal to the reference Main Commit.
4. Get the first Testing commit strictly on top of the previously
   chosen Main commit ID. If not found, use the previously chosen Main
   commit ID.
