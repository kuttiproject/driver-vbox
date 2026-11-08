package drivervbox

// findvboxmanage finds the VBoxManage tool. It is a variable so that tests can
// replace it. Its default, lookupvboxmanage, is made for each platform.
var findvboxmanage = lookupvboxmanage
