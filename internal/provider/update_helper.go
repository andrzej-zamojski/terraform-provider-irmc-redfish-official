/*
Copyright (c) 2025 Fsas Technologies Inc.,
or its subsidiaries. All Rights Reserved.

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
either express or implied.
*/

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"terraform-provider-irmc-redfish/internal/models"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/stmcginnis/gofish"
)

const CACHE_DURATION = 6 * time.Hour

type onlineUpdateEndpoints struct {
	checkEndpoint            string
	collectionEndpoint       string
	modifyCollectionEndpoint string
}

func GetOnlineUpdateEndpoints(isFsas bool) onlineUpdateEndpoints {
	if isFsas {
		return onlineUpdateEndpoints{
			checkEndpoint:            fmt.Sprintf("/redfish/v1/Systems/0/Oem/%s/eLCM/Actions/%seLCM.OnlineUpdate", FSAS, FSAS),
			collectionEndpoint:       fmt.Sprintf("/redfish/v1/Systems/0/Oem/%s/eLCM/Actions/%seLCM.OnlineUpdateGetCollection", FSAS, FSAS),
			modifyCollectionEndpoint: fmt.Sprintf("/redfish/v1/Systems/0/Oem/%s/eLCM/Actions/%seLCM.OnlineUpdateModifyCollection", FSAS, FSAS),
		}
	} else {
		return onlineUpdateEndpoints{
			checkEndpoint:            fmt.Sprintf("/redfish/v1/Systems/0/Oem/%s/eLCM/Actions/%seLCM.OnlineUpdate", TS_FUJITSU, FTS),
			collectionEndpoint:       fmt.Sprintf("/redfish/v1/Systems/0/Oem/%s/eLCM/Actions/%seLCM.OnlineUpdateGetCollection", TS_FUJITSU, FTS),
			modifyCollectionEndpoint: fmt.Sprintf("/redfish/v1/Systems/0/Oem/%s/eLCM/Actions/%seLCM.OnlineUpdateModifyCollection", TS_FUJITSU, FTS),
		}
	}
}

func GetLicenseEndpoint(isFsas bool) string {
	if isFsas {
		return fmt.Sprintf("/redfish/v1/Managers/iRMC/Oem/%s/iRMCConfiguration/Licenses", FSAS)
	}
	return fmt.Sprintf("/redfish/v1/Managers/iRMC/Oem/%s/iRMCConfiguration/Licenses", TS_FUJITSU)
}

func GetSystemOemEndpoint(isFsas bool) string {
	if isFsas {
		return fmt.Sprintf("/redfish/v1/Systems/0/Oem/%s/System", FSAS)
	}
	return fmt.Sprintf("/redfish/v1/Systems/0/Oem/%s/System", TS_FUJITSU)
}

func CheckELCMLicense(api *gofish.APIClient, endpoint string) error {
	resp, err := api.Service.GetClient().Get(endpoint)
	if err != nil {
		return fmt.Errorf("failed to get license info from %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d while fetching licenses: %s", resp.StatusCode, string(body))
	}

	var licenseInfo struct {
		Keys []struct {
			Name string `json:"Name"`
		} `json:"Keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&licenseInfo); err != nil {
		return fmt.Errorf("failed to decode license information: %w", err)
	}

	for _, key := range licenseInfo.Keys {
		if key.Name == "eLCM" {
			return nil
		}
	}

	return fmt.Errorf("eLCM license not found. Online update functionality requires an active eLCM license on the iRMC.")
}

func TriggerOnlineUpdateCheck(ctx context.Context, api *gofish.APIClient, endpoint string) (string, error) {
	client := api.Service.GetClient()
	payload := map[string]string{
		"ExecutionMode":  "CheckForUpdate",
		"SchedulingType": "Immediately",
	}

	respPost, err := client.Post(endpoint, payload)
	if err != nil {
		return "", fmt.Errorf("POST request failed: %w", err)
	}
	defer respPost.Body.Close()

	if respPost.StatusCode != http.StatusOK && respPost.StatusCode != http.StatusAccepted && respPost.StatusCode != http.StatusCreated && respPost.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(respPost.Body)
		return "", fmt.Errorf("Unexpected response status: %d, response body: %s", respPost.StatusCode, string(responseBody))
	}
	location := respPost.Header.Get(HTTP_HEADER_LOCATION)
	if location == "" {
		tflog.Warn(ctx, "Task Location header not found in response.")
		return "", nil
	}

	return location, nil
}

func GetOnlineUpdateCollection(ctx context.Context, api *gofish.APIClient, endpoint string) (*models.OnlineUpdateCheck, bool, error) {
	client := api.Service.GetClient()

	resp, err := client.Post(endpoint, map[string]string{})
	if err != nil {
		return nil, false, fmt.Errorf("POST request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("Unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Status               string `json:"Status"`
		LastStatusChangeDate string `json:"LastStatusChangeDate"`
		UpdateCollection     []struct {
			Designation  string `json:"Designation"`
			Component    string `json:"Component"`
			SubComponent string `json:"SubComponent"`
			Current      string `json:"Current"`
			New          string `json:"New"`
			Severity     string `json:"Severity"`
			Status       string `json:"Status"`
			Reboot       string `json:"Reboot"`
			Downloaded   bool   `json:"Downloaded"`
			Execution    string `json:"Execution"`
			RelNotePath  string `json:"RelNotePath"`
		} `json:"UpdateCollection"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, fmt.Errorf("Error decoding JSON response: %w", err)
	}

	if result.Status == "InProgress" {
		return nil, true, nil
	}

	var updates []models.OnlineUpdateCheckItem
	for _, u := range result.UpdateCollection {
		updates = append(updates, models.OnlineUpdateCheckItem{
			Designation:     types.StringValue(u.Designation),
			Component:       types.StringValue(u.Component),
			SubComponent:    types.StringValue(u.SubComponent),
			CurrentVersion:  types.StringValue(u.Current),
			NewVersion:      types.StringValue(u.New),
			Severity:        types.StringValue(u.Severity),
			Status:          types.StringValue(u.Status),
			RebootRequired:  types.StringValue(u.Reboot),
			Downloaded:      types.BoolValue(u.Downloaded),
			ExecutionStatus: types.StringValue(u.Execution),
			RelNotePath:     types.StringValue(u.RelNotePath),
		})
	}

	return &models.OnlineUpdateCheck{
		LastStatusChangeDate: types.StringValue(result.LastStatusChangeDate),
		UpdateCollection:     updates,
	}, false, nil
}

func GetOnlineUpdateCollectionWithRetry(ctx context.Context, api *gofish.APIClient, endpoint string, retries int, delay time.Duration) (*models.OnlineUpdateCheck, error) {
	for i := 0; i < retries; i++ {
		collection, inProgress, err := GetOnlineUpdateCollection(ctx, api, endpoint)
		if err != nil {
			return nil, err
		}

		if inProgress {
			time.Sleep(delay)
			continue
		}

		return collection, nil
	}
	return nil, fmt.Errorf("Collection was not ready after %d retries", retries)
}

func CheckOnlineUpdateStatus(ctx context.Context, service *gofish.Service, location string, timeout int64, isFsas bool) error {
	finishedSuccessfully, err := WaitForRedfishTaskEnd(ctx, service, location, timeout)
	if err != nil || !finishedSuccessfully {
		taskLog, diags := FetchRedfishTaskLog(service, location, isFsas)
		if diags.HasError() {
			return fmt.Errorf("Online update check task did not complete successfully: %v", err)
		}
		return fmt.Errorf("Oonline update check task failed. Details: %v. Task log: %s", err, string(taskLog))
	}
	return nil
}

func IsCollectionCacheValid(ctx context.Context, api *gofish.APIClient, collectionEndpoint string) bool {

	existingCollection, err := GetOnlineUpdateCollectionWithRetry(ctx, api, collectionEndpoint, 2, 1*time.Second)

	if err != nil {
		return false
	}

	if !existingCollection.LastStatusChangeDate.IsNull() && !existingCollection.LastStatusChangeDate.IsUnknown() {
		dateStr := existingCollection.LastStatusChangeDate.ValueString()
		lastCheckTime, parseErr := time.Parse(time.RFC3339, dateStr)
		if parseErr == nil {
			if time.Since(lastCheckTime) < CACHE_DURATION {
				return true
			}
		}
	}
	return false
}
