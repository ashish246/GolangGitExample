package uam

allow {
 userRole := data.userRoles[input.user] 
 resRole := data.resource2role[input.resource]
 userComb := data.roleCombination[uRole]
 resComb := data.roleCombination[eRole] 
 userComb[_]==resComb[_]
}
