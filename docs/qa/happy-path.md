# QA Steps

If this is an agent running, you will see "via `<command>`. Take the command within the backticks and run it literally. I.e., step 2 states to run:
`(cd /home/ubuntu/repos/juju && make install)` - run this in your integrated terminal, it will enable you to run commands in different directories.

1. If the patch for Juju was changed, install it via `(cd /home/ubuntu/repos/juju && make install)`, then the Juju CLI installed into go bin can be used for QA (including bootstrapping).
2. Spin up the compose, AIR will rebuild the server. Check it is running via `(cd /home/ubuntu/repos/test/remote-controller-store && docker compose ps)`. If it isn't, run `(/home/ubuntu/repos/test/remote-controller-store && docker compose up -d --wait)`.
2. If any services are unhealthy, stop, and attempt to diagnose.
3. Check if a controller called lol exists - if the change impacts controller auth from the controller token, a new controller will need creating. This can be done via `juju controllers`.
  If not, run `juju bootstrap lxd lol --config login-token-refresh-url=http://$(lxc network list --format json | jq -r '.[] | select(.name=="lxdbr0") | .config["ipv4.address"] | split("/")[0]'):8484/.well-known/jwks.json --verbose` Can be run anywhere.
4. If the CLI for RCS was modified, run `(cd /home/ubuntu/repos/test/remote-controller-store && make build link)` to create a new build, and symlink at `/home/ubuntu/repos/test/remote-controller-store/rcs` to the CLI in `/home/ubuntu/repos/test/remote-controller-store/build`. 
5. Once al is up and running, run `/home/ubuntu/repos/test/remote-controller-store/rcs logout` and `/home/ubuntu/repos/test/remote-controller-store/rcs login http://localhost:8484`. If this is an agent running this, prompt the user to go the URL presented. Once they login, continue :). 
6. RUn `/home/ubuntu/repos/test/remote-controller-store/rcs whoami` - verify we're logged in. 
7. Run `juju models` 
8. Run `juju add-model a`, if a already exists, move to the next letter.

Complete.






