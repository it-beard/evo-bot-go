package grouphandlersservices

import (
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type SaveTopicService struct {
	groupTopicRepository *repositories.GroupTopicRepository
}

func NewSaveTopicService(groupTopicRepository *repositories.GroupTopicRepository) *SaveTopicService {
	return &SaveTopicService{groupTopicRepository: groupTopicRepository}
}

func (s *SaveTopicService) SaveOrUpdateTopic(msg *gotgbot.Message) error {
	if msg.ForumTopicCreated != nil {
		// Handle forum topic creation
		return s.handleForumTopicCreated(msg)
	}

	if msg.ForumTopicEdited != nil {
		// Handle forum topic edit
		return s.handleForumTopicEdited(msg)
	}

	return nil
}

func (s *SaveTopicService) IsTopicShouldBeSavedOrUpdated(msg *gotgbot.Message) bool {
	// Handle forum topic created or edited messages
	return msg.ForumTopicCreated != nil || msg.ForumTopicEdited != nil
}

func (h *SaveTopicService) handleForumTopicCreated(msg *gotgbot.Message) error {
	topicCreated := msg.ForumTopicCreated
	topicID := msg.MessageThreadId
	topicName := topicCreated.Name

	log.Printf("%s: Forum topic created - ID: %d, Name: %s", utils.GetCurrentTypeName(), topicID, topicName)

	// Save the new topic to database
	groupTopic, err := h.groupTopicRepository.AddGroupTopic(topicID, topicName)
	if err != nil {
		return fmt.Errorf("%s: failed to save forum topic created: %w", utils.GetCurrentTypeName(), err)
	}

	log.Printf("%s: Successfully saved forum topic created - DB ID: %d, Topic ID: %d, Name: %s",
		utils.GetCurrentTypeName(), groupTopic.ID, groupTopic.TopicID, groupTopic.Name)

	return nil
}

func (h *SaveTopicService) handleForumTopicEdited(msg *gotgbot.Message) error {
	topicEdited := msg.ForumTopicEdited
	topicID := msg.MessageThreadId

	// The topic name might not change in edit, but we'll handle the case where it does
	var topicName string
	if topicEdited.Name != "" {
		topicName = topicEdited.Name
	} else {
		// If no name change, try to get existing topic
		existingTopic, err := h.groupTopicRepository.GetGroupTopicByTopicID(topicID)
		if err != nil {
			return fmt.Errorf("%s: failed to get existing topic for edit: %w", utils.GetCurrentTypeName(), err)
		}
		topicName = existingTopic.Name
	}

	log.Printf("%s: Forum topic edited - ID: %d, Name: %s", utils.GetCurrentTypeName(), topicID, topicName)

	// Update the topic in database
	groupTopic, err := h.groupTopicRepository.UpdateGroupTopic(topicID, topicName)
	if err != nil {
		// If topic doesn't exist, create it (edge case handling)
		if utils.IndexAny(err.Error(), "no group topic found") != -1 {
			log.Printf("%s: Topic not found during edit, creating new one - ID: %d, Name: %s",
				utils.GetCurrentTypeName(), topicID, topicName)
			groupTopic, err = h.groupTopicRepository.AddGroupTopic(topicID, topicName)
			if err != nil {
				return fmt.Errorf("%s: failed to create forum topic during edit: %w", utils.GetCurrentTypeName(), err)
			}
		} else {
			return fmt.Errorf("%s: failed to update forum topic edited: %w", utils.GetCurrentTypeName(), err)
		}
	}

	log.Printf("%s: Successfully updated forum topic edited - DB ID: %d, Topic ID: %d, Name: %s",
		utils.GetCurrentTypeName(), groupTopic.ID, groupTopic.TopicID, groupTopic.Name)

	return nil
}
