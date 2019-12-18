package uam2

default allow = false

allow {

    ldapGroups := data["uam"]["user2LdapGroups"][input.user];
	ldapGroup := ldapGroups[_];
	entGroup := data["uam"]["resource2EntGroup"][input.resource][_]; 
	data["uam"]["entGroup2LdapGroups"][entGroup][_] == ldapGroup
 
# data["uam"]["entGroup2LdapGroups"][data["uam"]["resource2EntGroup"][input.resource][_]][_] == data["uam"]["user2LdapGroups"][input.user][_]

}
