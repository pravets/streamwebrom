# Persistent rooms

[Español](../SALAS-PERSISTENTES.md) · **English**

Create a room with a password. In the host's studio press **Save room** in the top bar, or **Room** in the right rail, choose **30, 60, 90, 365 days or Unlimited** and press **Save room**. Persistence is not activated by creating the room or by entering it. The date starts from that save; saving again renews the term. Subsequent changes to scenes, stage order, brand, banners, names, audio and visibility are saved automatically. The OBS link, invitation and host destination options are also preserved; changing those options does not renew the term. Removing persistence deletes the saved file and keeps the current session open as a temporary room.

The server stores in `-data/rooms/` the configuration, the access key hashes, the OBS/WHIP/invitation keys, the participant registry and the known sources. The room password is stored as a bcrypt hash. The files contain production secrets, including the saved stream destinations: protect the directory and its backups; do not publish them or include them in the repository.

The room survives closing the tabs and restarting the server. The host returns from their saved studios on the landing page or with their host link. Keep that link: the guest password does not grant host control. On expiry, re-entry is prevented and the saved room is deleted. An unlimited room remains until its persistence is removed.

Each guest keeps their identity in the same browser. In the persistence panel, the host can copy a **Personal link** so that person can come back from another device. The password is still mandatory. Do not share that link between people: it represents an identity and one main camera. Two people with the same name are not the same identity. Expelling someone revokes their previous pass.

The view key, the main camera identifiers and the saved OBS links remain the same; sources reappear when their owners publish again. The host's main camera and screen use stable identifiers. Additional cameras keep their slots according to the order in which they are added. Connections, screen permissions, cameras, local files, recordings and running broadcasts are not kept: each person authorises their devices again, and local media files must be selected again. A broadcast is not restarted automatically.

Hiding a camera from the host hides its image both in the scenes and in the backstage frames of host and guests, while keeping its audio. It is a presentation control, not a revocation of access to the tracks of already authorised participants.
