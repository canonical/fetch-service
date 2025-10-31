.. _howto-test:

Test the fetch service locally
==============================

During development, you'll likely want to run the fetch service locally. This guide shows
you how to do so.

Generate certificates
---------------------

The first thing to do is to generate a self-signed certificate for the fetch service proxy.
This can be done using the following ``openssl`` commands::

   openssl genrsa -aes256 -passout pass:1 -out ca.key.pem 4096
   openssl rsa -passin pass:1 -in ca.key.pem -out ca.key.pem.tmp
   mv ca.key.pem.tmp ca.key.pem
   openssl req -subj "/CN=root@localhost" -key ca.key.pem -new -x509 -days 7300 -sha256 -extensions v3_ca -out ca.pem

Start the fetch service
-----------------------

You can run the fetch service itself with ``go run``::

   go run ./cmd/fetch --permissive-mode --cert=./ca.pem --key=ca.key.pem --spool=./spool --verbosity=debug

This will create a ``spool`` directory where artefacts will be stored. Debug verbosity
is especially useful when examining what the inspectors are doing.

Start a session
---------------

In another terminal, you can create a session with ``fetchctl``. You'll need to symlink
it to ``fetch``, as it's simply a different entrypoint for the same file::

   ln -s fetch fetchctl
   ./fetchctl create-session --session-id=abc --token=def --permissive

This creates a permissive session in the running fetch service, which can then be used
elsewhere.

The inspectors configuration for the session is provided as a optional positional argument.
Save the following file:

.. literalinclude:: code/inspectors.yaml
    :caption: inspectors.yaml
    :language: yaml

Then run::
        
   ./fetchctl create-session --session-id=abc --token=def --permissive inspectors.yaml


Use the session
---------------

You could load your self-signed certificate as an accepted certificate if you want, or you
could choose not to verify TLS. Set your proxy configuration with the session ID as the username
and the token as the password, and run a command. For example, you could test
by cloning a git repostiory::

   https_proxy=http://abc:def@localhost:9988 git -c http.sslVerify=false clone https://github.com/lengau/charmcraft-rocks --depth 1 --no-progress

The default proxy port is 9988, but if you set something different when running the fetch
service above, adjust accordingly.
