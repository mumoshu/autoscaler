/*
Copyright 2016 The Kubernetes Authors.

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

package aws

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/golang/glog"
)

// autoScaling is the interface represents a specific aspect of the auto-scaling service provided by AWS SDK for use in CA
type autoScaling interface {
	DescribeAutoScalingGroups(input *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
	DescribeTags(input *autoscaling.DescribeTagsInput) (*autoscaling.DescribeTagsOutput, error)
	SetDesiredCapacity(input *autoscaling.SetDesiredCapacityInput) (*autoscaling.SetDesiredCapacityOutput, error)
	TerminateInstanceInAutoScalingGroup(input *autoscaling.TerminateInstanceInAutoScalingGroupInput) (*autoscaling.TerminateInstanceInAutoScalingGroupOutput, error)
}

// autoScalingWrapper provides several utility methods over the auto-scaling service provided by AWS SDK
type autoScalingWrapper struct {
	autoScaling
}

func (m autoScalingWrapper) getAutoscalingGroupByName(name string) (*autoscaling.Group, error) {
	params := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{aws.String(name)},
		MaxRecords:            aws.Int64(1),
	}
	groups, err := m.DescribeAutoScalingGroups(params)
	if err != nil {
		glog.V(4).Infof("Failed ASG info request for %s: %v", name, err)
		return nil, err
	}
	if len(groups.AutoScalingGroups) < 1 {
		return nil, fmt.Errorf("Unable to get first autoscaling.Group for %s", name)
	}
	return groups.AutoScalingGroups[0], nil
}

func (m *autoScalingWrapper) getAutoscalingGroupsByNames(names []string) ([]*autoscaling.Group, error) {
	glog.V(6).Infof("Starting getAutoscalingGroupsByNames with names=%v", names)

	nameRefs := []*string{}
	for _, n := range names {
		nameRefs = append(nameRefs, aws.String(n))
	}
	params := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: nameRefs,
		MaxRecords:            aws.Int64(maxRecordsReturnedByAPI),
	}
	description, err := m.DescribeAutoScalingGroups(params)
	if err != nil {
		glog.V(4).Infof("Failed to describe ASGs : %v", err)
		return nil, err
	}
	if len(description.AutoScalingGroups) < 1 {
		return nil, errors.New("No ASGs found")
	}

	asgs := description.AutoScalingGroups
	for description.NextToken != nil {
		description, err = m.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
			NextToken:  description.NextToken,
			MaxRecords: aws.Int64(maxRecordsReturnedByAPI),
		})
		if err != nil {
			glog.V(4).Infof("Failed to describe ASGs : %v", err)
			return nil, err
		}
		asgs = append(asgs, description.AutoScalingGroups...)
	}

	glog.V(6).Infof("Finishing getAutoscalingGroupsByNames asgs=%v", asgs)

	return asgs, nil
}

func (m *autoScalingWrapper) getAutoscalingGroupsByTag(key string) ([]*autoscaling.Group, error) {
	glog.V(6).Infof("Starting getAutoscalingGroupsByTag with key=%v", key)

	tags := []*autoscaling.TagDescription{}

	description, err := m.DescribeTags(&autoscaling.DescribeTagsInput{
		Filters: []*autoscaling.Filter{
			{
				Name:   aws.String("key"),
				Values: []*string{aws.String(key)},
			},
		},
		MaxRecords: aws.Int64(maxRecordsReturnedByAPI),
	})
	if err != nil {
		glog.V(4).Infof("Failed to describe ASG tags for key %s : %v", key, err)
		return nil, err
	}
	if len(description.Tags) < 1 {
		return nil, fmt.Errorf("Unable to find ASGs for tag key %s", key)
	}
	tags = append(tags, description.Tags...)

	for description.NextToken != nil {
		description, err = m.DescribeTags(&autoscaling.DescribeTagsInput{
			NextToken:  description.NextToken,
			MaxRecords: aws.Int64(maxRecordsReturnedByAPI),
		})
		if err != nil {
			glog.V(4).Infof("Failed to describe ASG tags for key %s: %v", key, err)
			return nil, err
		}
		tags = append(tags, description.Tags...)
	}

	asgNames := []string{}
	for _, t := range tags {
		asgName := t.ResourceId
		asgNames = append(asgNames, *asgName)
	}

	asgs, err := m.getAutoscalingGroupsByNames(asgNames)
	if err != nil {
		return nil, err
	}

	glog.V(6).Infof("Finishing getAutoscalingGroupsByTag with asgs=%v", asgs)

	return asgs, nil
}
