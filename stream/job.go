package stream

import (
	"github.com/google/uuid"
)

type JobUUID = uuid.UUID

// PRINCIPLE:
//
// In job est un code exécuté de manière asynchrone (car il peut mettre beaucoup de temps à s'exécuter)
// Le résultat est stocké sous forme de fichier (comme pour Athena)
// Une notification optionelle peut être envoyée pour signifier la fin du traitement
//
// USE CASES:
//
// 1: être capable d'uploader un gros fichier .jsonl contenant 1M de lignes vers le flux (multipart)
// 2: être capable de downloader un gros fichier .jsonl contenant tout le flux (sans passer par l'api flux)
//    - serveur web static files (pouvoir télécharger plusieurs fois le fichier)
//    - garder un historique (quota disk) des fichiers générés
// 3: être capable de downloader un fichier .jsonl contenant le flux filtré (subset) (jq filter)
//    - ex: filtrer le flux pour ne garder que les data du 4 mars 2021
//    - ex: filtrer le flux pour ne garder que les data concenant les erreurs ("level" = "ERROR")
// 4: être capable de downloader un fichier .jsonl contenant le flux aggrégé
//    - ex: filtrer le flux pour ne garder que le nombre d'erreurs (filter) cumulé (sum), groupé par heure (aggr), du 4 mars 2021 (filter)
//
// INSPIRATION:
//
// - jq (pour le filtrage et le remapage)
// - syntaxe filtres elasticsearch (pour l'aggregation)
// - kinesis (api stream)
// - kinesis firehose (api stream) (bucketting)
// - nginx (static files web serve)
// - filebeat (go, config)
// - datadog (saas)
// - kubernetes (jobs & cronjobs)

type Job struct {
	UUID JobUUID `json:"uuid" example:"4ce589e2-b483-467b-8b59-758b339801db"`
	// stream uuid ref
	// cron [optional]
	// jq filter [optional]
	// aggregation [optional]
	// urlNotification [optional]
	// expiresAfter [optional]
}

type JobMap = map[JobUUID]Job

type JobsCollection struct {
	Hashmap JobMap
}

var Jobs JobsCollection

func GetJob(uuid uuid.UUID) *Job {
	if job, found := Jobs.Hashmap[uuid]; found {
		return &job
	}

	return nil
}
