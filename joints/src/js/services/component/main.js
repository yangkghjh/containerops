/*
Copyright 2014 Huawei Technologies Co., Ltd. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
 */

function componentService(componentApiService){

	function getComponents(filterName,filterVersion,fuzzy,pageNum,versionNum,offset){
		var params = {
			"filterName" : filterName,
			"filterVersion" : filterVersion,
			"fuzzy" : fuzzy,
			"pageNum" : pageNum,
			"versionNum" : versionNum,
			"offset" : offset
		}
		return componentApiService.ajaxCall("list",params);
	}
	
	return {
		"getComponents" : getComponents
	}
}
   
devops.factory('componentService', ['componentApiService',componentService]);