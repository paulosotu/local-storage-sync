
#!/bin/ash

echo "$PRIVATE_CERT" > /root/.ssh/id_rsa
echo "$PUBLIC_CERT" > /root/.ssh/authorized_keys
echo "$PUBLIC_CERT" > /root/.ssh/id_rsa.pub
chmod 700 /root/.ssh/authorized_keys
chmod 700 /root/.ssh/id_rsa
passwd -u root

/usr/sbin/sshd -D &

echo running: /bin/local-storage-sync -c -app $APP_NAME -l $LOG_LEVEL -t $UPDATE_INTERVAL_SECONDS -d $SHARED_STORAGE_FOLDER -n $NODE_HOSTNAME 
/bin/local-storage-sync -c -app $APP_NAME -l $LOG_LEVEL -t $UPDATE_INTERVAL_SECONDS -d $SHARED_STORAGE_FOLDER -n $NODE_HOSTNAME 