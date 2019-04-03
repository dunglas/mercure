Use the following file as template to run mercure as supervisor-instance on a linux-system:

    [program:mercure]
    command=/path/to/mercure
    process_name=%(program_name)s_%(process_num)s
    numprocs=1
    environment=JWT_KEY="asdf",ADDR="my.host:3000",ALLOW_ANONYMOUS="0",CORS_ALLOWED_ORIGINS="http://my.host",PUBLISH_ALLOWED_ORIGINS="http://my.host",DEBUG="0"
    directory=/tmp
    autostart=true
    autorestart=true
    startsecs=5
    startretries=10
    user=www-data
    redirect_stderr=false
    stdout_capture_maxbytes=1MB
    stderr_capture_maxbytes=1MB
    stdout_logfile=/path/to/logs/mercure.out.log
    stderr_logfile=/path/to/logs/mercure.error.log
    
Save file to `/etc/supervisor/conf.d/mercure.conf`. Run `supervisorctl reread` and `supervisorctl update` to activate and start mercure.
