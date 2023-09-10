# Terraform How To
note: you must install terraform first on your system operation host

## if you are first setting this projects run 
go to specific project folder terraform rename  `terraform.tfvars.sample` to `terraform.tfvars` and fill the config and run.
```
terraform init
```
and then run if want to apply or creating the VM
```
terraform apply
```

and don't forget to 
```
terraform destroy
```
to destory the vm so you will not get high charge pricing.

## TODO
* set terraform output so can get the new IP Address assigned to specific VM
* 