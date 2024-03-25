# devstats-landscape-sync
📈🌄 Check if cncf/landscape projects data is in sync with cncf/devstats and report if it isn't via email


# Running

- `` clear && make && [LANDSCAPE_YAML_PATH=url|path] [PROJECTS_YAML_PATH=url|path] [DOCKER_PROJECTS_YAML_PATH=url|path] [EMAIL_TO=alerting-address@domain.com,alerting2@other.pl] [SKIP_EMAIL=1] ./check_sync ``.
- `` [DBG=1] ./check_sync.sh ``.


# Deploying

- Please use `check_sync.crontab` example cron deployment.
