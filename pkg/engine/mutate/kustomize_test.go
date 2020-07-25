package mutate

import (
	"strings"
	"testing"

	"gotest.tools/assert"
)

// note:         emptyDir: {} is removed from patch

func TestMergePatch(t *testing.T) {
	out, err := strategicMergePatchfilter()

	assert.NilError(t, err)
	assert.Equal(t, strings.TrimSpace(expect), strings.TrimSpace(out))
}

var base = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: wordpress
  labels:
    app: wordpress
spec:
  selector:
    matchLabels:
      app: wordpress
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: wordpress
    spec:
      containers:
      - image: wordpress:4.8-apache
        name: wordpress
        ports:
        - containerPort: 80
          name: wordpress
        volumeMounts:
        - name: wordpress-persistent-storage
          mountPath: /var/www/html
      volumes:
      - name: wordpress-persistent-storage
        emptyDir: {}
`

var overlay = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: wordpress
spec:
  template:
    spec:
      initContainers:
      - name: init-command
        image: debian
        command:
        - "echo $(WORDPRESS_SERVICE)"
        - "echo $(MYSQL_SERVICE)"
      containers:
      - name: nginx
        image: nginx
      - name: wordpress
        env:
        - name: WORDPRESS_DB_HOST
          value: $(MYSQL_SERVICE)
        - name: WORDPRESS_DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: mysql-pass
              key: password
`

var expect = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: wordpress
  labels:
    app: wordpress
spec:
  selector:
    matchLabels:
      app: wordpress
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: wordpress
    spec:
      containers:
      - image: wordpress:4.8-apache
        name: wordpress
        ports:
        - containerPort: 80
          name: wordpress
        volumeMounts:
        - name: wordpress-persistent-storage
          mountPath: /var/www/html
        env:
        - name: WORDPRESS_DB_HOST
          value: $(MYSQL_SERVICE)
        - name: WORDPRESS_DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: mysql-pass
              key: password
      - name: nginx
        image: nginx
      volumes:
      - name: wordpress-persistent-storage
        emptyDir: {}
      initContainers:
      - name: init-command
        image: debian
        command:
        - "echo $(WORDPRESS_SERVICE)"
        - "echo $(MYSQL_SERVICE)"
`
