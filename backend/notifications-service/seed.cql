-- Rucno popunjavanje baze notifikacijama (zahtev 1.7, ocena 6).
--
-- Najlakse je preko servisa (ne treba nista menjati u ovom fajlu):
--   docker exec google-drive-notifications-service ./notifications-service -seed <ID_KORISNIKA>
--
-- Ako hoces rucno kroz cqlsh: zameni USER_ID_OVDE pravim _id-jem korisnika
-- iz Compass-a (baza google_drive, kolekcija users), pa pokreni:
--   PowerShell:  Get-Content seed.cql | docker exec -i google-drive-cassandra cqlsh
--   Git Bash:    docker exec -i google-drive-cassandra cqlsh < seed.cql

USE notifications_db;

INSERT INTO notifications_by_user (user_id, created_at, notification_id, message, is_read)
VALUES ('USER_ID_OVDE', toTimestamp(now()), now(), 'Dobrodosli na Disk!', true);

INSERT INTO notifications_by_user (user_id, created_at, notification_id, message, is_read)
VALUES ('USER_ID_OVDE', toTimestamp(now()), now(), 'Otpremljena je nova verzija datoteke.', false);

SELECT * FROM notifications_by_user WHERE user_id = 'USER_ID_OVDE';
