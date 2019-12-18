package uam2

default allow = false

allow {

    ldapGroups := data["user2LdapGroups"][input.user];
	ldapGroup := ldapGroups[_];
	entGroup := data["resource2EntGroup"][input.resource][_]; 
	data["entGroup2LdapGroups"][entGroup][_] == ldapGroup
 
# data["uam"]["entGroup2LdapGroups"][data["uam"]["resource2EntGroup"][input.resource][_]][_] == data["uam"]["user2LdapGroups"][input.user][_]

}
