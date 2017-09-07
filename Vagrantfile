Vagrant.configure("2") do |config|
  config.vm.box = "alpine/alpine64"

  config.vm.provider "virtualbox" do |v|
    v.customize ["modifyvm", :id, "--memory", "2048"]
    v.customize ["modifyvm", :id, "--cpus", 2]
  end

 config.vm.synced_folder ".", "/home/vagrant/code", create: true, type: 'rsync'

 config.vm.provision "shell", inline: %q{

sed  -i "/edge\/community/s/^#//" /etc/apk/repositories

apk update
apk upgrade
apk add virtualbox-guest-additions
apk add go

cat > /bin/shutdown <<EOF
#!/bin/sh
poweroff
EOF

chmod a+x /bin/shutdown

}


end
