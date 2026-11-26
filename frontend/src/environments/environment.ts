// Sva komunikacija sa serverom ide kroz API gateway (2.2), preko HTTPS-a (2.19).
// Gateway koristi samopotpisan sertifikat: pre prvog rada u browseru otvori
// https://localhost:8080/health i prihvati ga, inace ce svi zahtevi pasti.
const gatewayUrl = 'https://localhost:8080';

export const environment = {
  apiUrl: `${gatewayUrl}/`,

  usersApiUrl: `${gatewayUrl}/api/users`,
  filesApiUrl: `${gatewayUrl}/api/files`,
  notificationsApiUrl: `${gatewayUrl}/api/notifications`,

  /** @deprecated Identitet sada dolazi iz JWT tokena; ne koristiti. */
  devUserId: 'dev-user-1',
};
