//  Copyright 2023 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/harness/gitness/app/url"
	artifactapi "github.com/harness/gitness/registry/app/api/openapi/contracts/artifact"
	"github.com/harness/gitness/registry/app/store/database"
	"github.com/harness/gitness/registry/types"

	"github.com/rs/zerolog/log"
)

func GetArtifactMetadata(
	ctx context.Context,
	artifacts []types.ArtifactMetadata,
	rootIdentifier string,
	urlProvider url.Provider,
) []artifactapi.ArtifactMetadata {
	artifactMetadataList := make([]artifactapi.ArtifactMetadata, 0, len(artifacts))
	for _, artifact := range artifacts {
		registryURL := urlProvider.RegistryURL(ctx, rootIdentifier, artifact.RepoName)
		if artifact.PackageType == artifactapi.PackageTypeGENERIC {
			registryURL = urlProvider.RegistryURL(ctx, rootIdentifier, "generic", artifact.RepoName)
		}
		artifactMetadata := mapToArtifactMetadata(artifact, registryURL)
		artifactMetadataList = append(artifactMetadataList, *artifactMetadata)
	}
	return artifactMetadataList
}

func GetRegistryArtifactMetadata(artifacts []types.ArtifactMetadata) []artifactapi.RegistryArtifactMetadata {
	artifactMetadataList := make([]artifactapi.RegistryArtifactMetadata, 0, len(artifacts))
	for _, artifact := range artifacts {
		artifactMetadata := mapToRegistryArtifactMetadata(artifact)
		artifactMetadataList = append(artifactMetadataList, *artifactMetadata)
	}
	return artifactMetadataList
}

func GetMavenArtifactDetail(
	image *types.Image, artifact *types.Artifact,
	metadata database.MavenMetadata,
) artifactapi.ArtifactDetail {
	createdAt := GetTimeInMs(artifact.CreatedAt)
	modifiedAt := GetTimeInMs(artifact.UpdatedAt)
	var size int64
	for _, file := range metadata.Files {
		size += file.Size
	}
	sizeVal := GetSize(size)
	artifactDetail := &artifactapi.ArtifactDetail{
		CreatedAt:  &createdAt,
		ModifiedAt: &modifiedAt,
		Name:       &image.Name,
		Version:    artifact.Version,
		Size:       &sizeVal,
	}
	return *artifactDetail
}

func mapToArtifactMetadata(
	artifact types.ArtifactMetadata,
	registryURL string,
) *artifactapi.ArtifactMetadata {
	lastModified := GetTimeInMs(artifact.ModifiedAt)
	packageType := artifact.PackageType
	pullCommand := GetPullCommand(artifact.Name, artifact.Version,
		string(packageType), registryURL)
	return &artifactapi.ArtifactMetadata{
		RegistryIdentifier: artifact.RepoName,
		Name:               artifact.Name,
		Version:            &artifact.Version,
		Labels:             &artifact.Labels,
		LastModified:       &lastModified,
		PackageType:        &packageType,
		DownloadsCount:     &artifact.DownloadCount,
		PullCommand:        &pullCommand,
	}
}

func mapToRegistryArtifactMetadata(artifact types.ArtifactMetadata) *artifactapi.RegistryArtifactMetadata {
	lastModified := GetTimeInMs(artifact.ModifiedAt)
	packageType := artifact.PackageType
	return &artifactapi.RegistryArtifactMetadata{
		RegistryIdentifier: artifact.RepoName,
		Name:               artifact.Name,
		LatestVersion:      artifact.LatestVersion,
		Labels:             &artifact.Labels,
		LastModified:       &lastModified,
		PackageType:        &packageType,
		DownloadsCount:     &artifact.DownloadCount,
	}
}

func toPackageType(packageTypeStr string) (artifactapi.PackageType, error) {
	switch packageTypeStr {
	case string(artifactapi.PackageTypeDOCKER):
		return artifactapi.PackageTypeDOCKER, nil
	case string(artifactapi.PackageTypeGENERIC):
		return artifactapi.PackageTypeGENERIC, nil
	case string(artifactapi.PackageTypeHELM):
		return artifactapi.PackageTypeHELM, nil
	case string(artifactapi.PackageTypeMAVEN):
		return artifactapi.PackageTypeMAVEN, nil
	default:
		return "", errors.New("invalid package type")
	}
}

func GetTagMetadata(
	ctx context.Context,
	tags *[]types.TagMetadata,
	image string,
	registryURL string,
) []artifactapi.ArtifactVersionMetadata {
	artifactVersionMetadataList := []artifactapi.ArtifactVersionMetadata{}
	for _, tag := range *tags {
		modifiedAt := GetTimeInMs(tag.ModifiedAt)
		size := GetImageSize(tag.Size)
		digestCount := tag.DigestCount
		command := GetPullCommand(image, tag.Name, string(tag.PackageType), registryURL)
		packageType, err := toPackageType(string(tag.PackageType))
		downloadCount := tag.DownloadCount
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Error converting package type %s", tag.PackageType)
			continue
		}
		artifactVersionMetadata := &artifactapi.ArtifactVersionMetadata{
			PackageType:    &packageType,
			Name:           tag.Name,
			Size:           &size,
			LastModified:   &modifiedAt,
			DigestCount:    &digestCount,
			PullCommand:    &command,
			DownloadsCount: &downloadCount,
		}
		artifactVersionMetadataList = append(artifactVersionMetadataList, *artifactVersionMetadata)
	}
	return artifactVersionMetadataList
}

func GetAllArtifactResponse(
	ctx context.Context,
	artifacts *[]types.ArtifactMetadata,
	count int64,
	pageNumber int64,
	pageSize int,
	rootIdentifier string,
	urlProvider url.Provider,
) *artifactapi.ListArtifactResponseJSONResponse {
	var artifactMetadataList []artifactapi.ArtifactMetadata
	if artifacts == nil {
		artifactMetadataList = make([]artifactapi.ArtifactMetadata, 0)
	} else {
		artifactMetadataList = GetArtifactMetadata(ctx, *artifacts, rootIdentifier, urlProvider)
	}
	pageCount := GetPageCount(count, pageSize)
	listArtifact := &artifactapi.ListArtifact{
		ItemCount: &count,
		PageCount: &pageCount,
		PageIndex: &pageNumber,
		PageSize:  &pageSize,
		Artifacts: artifactMetadataList,
	}
	response := &artifactapi.ListArtifactResponseJSONResponse{
		Data:   *listArtifact,
		Status: artifactapi.StatusSUCCESS,
	}
	return response
}

func GetAllArtifactFilesResponse(
	files *[]types.FileNodeMetadata,
	count int64,
	pageNumber int64,
	pageSize int,
	registryURL string,
	artifactName string,
	version string,
	packageType artifactapi.PackageType,
) *artifactapi.FileDetailResponseJSONResponse {
	var fileMetadataList []artifactapi.FileDetail
	if files == nil {
		fileMetadataList = make([]artifactapi.FileDetail, 0)
	} else {
		fileMetadataList = GetArtifactFilesMetadata(files, registryURL, artifactName, version, packageType)
	}
	pageCount := GetPageCount(count, pageSize)
	return &artifactapi.FileDetailResponseJSONResponse{
		ItemCount: &count,
		PageCount: &pageCount,
		PageIndex: &pageNumber,
		PageSize:  &pageSize,
		Files:     fileMetadataList,
		Status:    "SUCCESS",
	}
}

func GetArtifactFilesMetadata(
	metadata *[]types.FileNodeMetadata,
	registryURL string,
	artifactName string,
	version string,
	packageType artifactapi.PackageType,
) []artifactapi.FileDetail {
	var files []artifactapi.FileDetail
	for _, file := range *metadata {
		filePathPrefix := "/" + artifactName + "/" + version + "/"
		filename := strings.Replace(file.Path, filePathPrefix, "", 1)
		var downloadCommand string
		if artifactapi.PackageTypeGENERIC == packageType {
			downloadCommand = GetGenericArtifactFileDownloadCommand(registryURL, artifactName, version, filename)
		} else if artifactapi.PackageTypeMAVEN == packageType {
			artifactName = strings.ReplaceAll(artifactName, ".", "/")
			artifactName = strings.ReplaceAll(artifactName, ":", "/")
			filePathPrefix = "/" + artifactName + "/" + version + "/"
			filename = strings.Replace(file.Path, filePathPrefix, "", 1)
			downloadCommand = GetMavenArtifactFileDownloadCommand(registryURL, artifactName, version, filename)
		}
		files = append(files, artifactapi.FileDetail{
			Checksums:       getCheckSums(file),
			Size:            GetSize(file.Size),
			CreatedAt:       fmt.Sprint(file.CreatedAt),
			Name:            filename,
			DownloadCommand: downloadCommand,
		})
	}
	return files
}

func getCheckSums(file types.FileNodeMetadata) []string {
	return []string{
		fmt.Sprintf("SHA-512: %s", file.Sha512),
		fmt.Sprintf("SHA-256: %s", file.Sha256),
		fmt.Sprintf("SHA-1: %s", file.Sha1),
		fmt.Sprintf("MD5: %s", file.MD5),
	}
}

func GetAllArtifactByRegistryResponse(
	artifacts *[]types.ArtifactMetadata,
	count int64,
	pageNumber int64,
	pageSize int,
) *artifactapi.ListRegistryArtifactResponseJSONResponse {
	var artifactMetadataList []artifactapi.RegistryArtifactMetadata
	if artifacts == nil {
		artifactMetadataList = make([]artifactapi.RegistryArtifactMetadata, 0)
	} else {
		artifactMetadataList = GetRegistryArtifactMetadata(*artifacts)
	}
	pageCount := GetPageCount(count, pageSize)
	listArtifact := &artifactapi.ListRegistryArtifact{
		ItemCount: &count,
		PageCount: &pageCount,
		PageIndex: &pageNumber,
		PageSize:  &pageSize,
		Artifacts: artifactMetadataList,
	}
	response := &artifactapi.ListRegistryArtifactResponseJSONResponse{
		Data:   *listArtifact,
		Status: artifactapi.StatusSUCCESS,
	}
	return response
}

func GetAllArtifactLabelsResponse(
	artifactLabels *[]string,
	count int64,
	pageNumber int64,
	pageSize int,
) *artifactapi.ListArtifactLabelResponseJSONResponse {
	pageCount := GetPageCount(count, pageSize)
	listArtifactLabels := &artifactapi.ListArtifactLabel{
		ItemCount: &count,
		PageCount: &pageCount,
		PageIndex: &pageNumber,
		PageSize:  &pageSize,
		Labels:    *artifactLabels,
	}
	response := &artifactapi.ListArtifactLabelResponseJSONResponse{
		Data:   *listArtifactLabels,
		Status: artifactapi.StatusSUCCESS,
	}
	return response
}

func GetAllArtifactVersionResponse(
	ctx context.Context,
	tags *[]types.TagMetadata,
	image string,
	count int64,
	pageNumber int64,
	pageSize int,
	registryURL string,
) *artifactapi.ListArtifactVersionResponseJSONResponse {
	artifactVersionMetadataList := GetTagMetadata(
		ctx, tags, image, registryURL,
	)
	pageCount := GetPageCount(count, pageSize)
	listArtifactVersions := &artifactapi.ListArtifactVersion{
		ItemCount:        &count,
		PageCount:        &pageCount,
		PageIndex:        &pageNumber,
		PageSize:         &pageSize,
		ArtifactVersions: &artifactVersionMetadataList,
	}
	response := &artifactapi.ListArtifactVersionResponseJSONResponse{
		Data:   *listArtifactVersions,
		Status: artifactapi.StatusSUCCESS,
	}
	return response
}

func GetNonOCIAllArtifactVersionResponse(
	ctx context.Context,
	artifacts *[]types.NonOCIArtifactMetadata,
	image string,
	count int64,
	pageNumber int64,
	pageSize int,
	registryURL string,
) *artifactapi.ListArtifactVersionResponseJSONResponse {
	artifactVersionMetadataList := GetNonOCIArtifactMetadata(
		ctx, artifacts, image, registryURL,
	)
	pageCount := GetPageCount(count, pageSize)
	listArtifactVersions := &artifactapi.ListArtifactVersion{
		ItemCount:        &count,
		PageCount:        &pageCount,
		PageIndex:        &pageNumber,
		PageSize:         &pageSize,
		ArtifactVersions: &artifactVersionMetadataList,
	}
	response := &artifactapi.ListArtifactVersionResponseJSONResponse{
		Data:   *listArtifactVersions,
		Status: artifactapi.StatusSUCCESS,
	}
	return response
}

func GetNonOCIArtifactMetadata(
	ctx context.Context,
	tags *[]types.NonOCIArtifactMetadata,
	image string,
	registryURL string,
) []artifactapi.ArtifactVersionMetadata {
	artifactVersionMetadataList := []artifactapi.ArtifactVersionMetadata{}
	for _, tag := range *tags {
		modifiedAt := GetTimeInMs(tag.ModifiedAt)
		size := GetImageSize(tag.Size)
		command := GetPullCommand(image, tag.Name, string(tag.PackageType), registryURL)
		packageType, err := toPackageType(string(tag.PackageType))
		downloadCount := tag.DownloadCount
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Error converting package type %s", tag.PackageType)
			continue
		}
		fileCount := tag.FileCount
		artifactVersionMetadata := &artifactapi.ArtifactVersionMetadata{
			PackageType:    &packageType,
			FileCount:      &fileCount,
			Name:           tag.Name,
			Size:           &size,
			LastModified:   &modifiedAt,
			PullCommand:    &command,
			DownloadsCount: &downloadCount,
		}
		artifactVersionMetadataList = append(artifactVersionMetadataList, *artifactVersionMetadata)
	}
	return artifactVersionMetadataList
}

func GetDockerArtifactDetails(
	registry *types.Registry,
	tag *types.TagDetail,
	manifest *types.Manifest,
	registryURL string,
) *artifactapi.DockerArtifactDetailResponseJSONResponse {
	repoPath := getRepoPath(registry.Name, tag.ImageName, manifest.Digest.String())
	pullCommand := GetDockerPullCommand(tag.ImageName, tag.Name, registryURL)
	createdAt := GetTimeInMs(tag.CreatedAt)
	modifiedAt := GetTimeInMs(tag.UpdatedAt)
	size := GetSize(manifest.TotalSize)
	artifactDetail := &artifactapi.DockerArtifactDetail{
		ImageName:      tag.ImageName,
		Version:        tag.Name,
		PackageType:    registry.PackageType,
		CreatedAt:      &createdAt,
		ModifiedAt:     &modifiedAt,
		RegistryPath:   repoPath,
		PullCommand:    &pullCommand,
		Url:            GetTagURL(tag.ImageName, tag.Name, registryURL),
		Size:           &size,
		DownloadsCount: &tag.DownloadCount,
	}

	response := &artifactapi.DockerArtifactDetailResponseJSONResponse{
		Data:   *artifactDetail,
		Status: artifactapi.StatusSUCCESS,
	}
	return response
}

func GetHelmArtifactDetails(
	registry *types.Registry,
	tag *types.TagDetail,
	manifest *types.Manifest,
	registryURL string,
) *artifactapi.HelmArtifactDetailResponseJSONResponse {
	repoPath := getRepoPath(registry.Name, tag.ImageName, manifest.Digest.String())
	pullCommand := GetHelmPullCommand(tag.ImageName, tag.Name, registryURL)
	createdAt := GetTimeInMs(tag.CreatedAt)
	modifiedAt := GetTimeInMs(tag.UpdatedAt)
	size := GetSize(manifest.TotalSize)
	downloadCount := tag.DownloadCount
	artifactDetail := &artifactapi.HelmArtifactDetail{
		Artifact:       &tag.ImageName,
		Version:        tag.Name,
		PackageType:    registry.PackageType,
		CreatedAt:      &createdAt,
		ModifiedAt:     &modifiedAt,
		RegistryPath:   repoPath,
		PullCommand:    &pullCommand,
		Url:            GetTagURL(tag.ImageName, tag.Name, registryURL),
		Size:           &size,
		DownloadsCount: &downloadCount,
	}

	response := &artifactapi.HelmArtifactDetailResponseJSONResponse{
		Data:   *artifactDetail,
		Status: artifactapi.StatusSUCCESS,
	}
	return response
}

func GetGenericArtifactDetail(
	image *types.Image, artifact *types.Artifact,
	metadata database.GenericMetadata,
) artifactapi.ArtifactDetail {
	createdAt := GetTimeInMs(artifact.CreatedAt)
	modifiedAt := GetTimeInMs(artifact.UpdatedAt)
	artifactDetail := &artifactapi.ArtifactDetail{
		CreatedAt:  &createdAt,
		ModifiedAt: &modifiedAt,
		Name:       &image.Name,
		Version:    artifact.Version,
	}
	err := artifactDetail.FromGenericArtifactDetailConfig(artifactapi.GenericArtifactDetailConfig{
		Description: &metadata.Description,
	})
	if err != nil {
		return artifactapi.ArtifactDetail{}
	}
	return *artifactDetail
}

func GetArtifactSummary(artifact types.ArtifactMetadata) *artifactapi.ArtifactSummaryResponseJSONResponse {
	createdAt := GetTimeInMs(artifact.CreatedAt)
	modifiedAt := GetTimeInMs(artifact.ModifiedAt)
	artifactVersionSummary := &artifactapi.ArtifactSummary{
		CreatedAt:      &createdAt,
		ModifiedAt:     &modifiedAt,
		DownloadsCount: &artifact.DownloadCount,
		ImageName:      artifact.Name,
		Labels:         &artifact.Labels,
		PackageType:    artifact.PackageType,
	}
	response := &artifactapi.ArtifactSummaryResponseJSONResponse{
		Data:   *artifactVersionSummary,
		Status: artifactapi.StatusSUCCESS,
	}
	return response
}

func GetArtifactVersionSummary(
	artifactName string,
	packageType artifactapi.PackageType,
	version string,
) *artifactapi.ArtifactVersionSummaryResponseJSONResponse {
	artifactVersionSummary := &artifactapi.ArtifactVersionSummary{
		ImageName:   artifactName,
		PackageType: packageType,
		Version:     version,
	}
	response := &artifactapi.ArtifactVersionSummaryResponseJSONResponse{
		Data:   *artifactVersionSummary,
		Status: artifactapi.StatusSUCCESS,
	}
	return response
}

func getRepoPath(registry string, image string, tag string) string {
	return filepath.Join(registry, image, tag)
}
