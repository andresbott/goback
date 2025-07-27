#ssh

This library is a thin utility wrapper around ssh actions like ssh commands, scp and sfpt

structured in different parts:
* ssh holds the basic logic to establish a connection to a server and create sessions
* scp utilizes the scp library and adds some utility functions like walk (incomplete work for now)

### ssh

``` go		
cl, _ := New(Cfg{
    Host:          sshServer.host,
    Port:          sshServer.port,
    Auth:          Password,
    User:          "pwuser",
    Password:      "1234",
    IgnoreHostKey: true,
})

// connect to the server
cl.Connect()

// disconnect once all the actions have been performed 
defer cl.Disconnect()

// get a session
session, _ := s.Session()
// close the session, only one operation allowed per session because it's not interactive
defer session.Close()

// run command and capture stdout/stderr
output, _ := session.CombinedOutput(cmd)

```