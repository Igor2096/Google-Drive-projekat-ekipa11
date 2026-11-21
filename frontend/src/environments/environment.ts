export const environment = {
  // Dok ne proradi API gateway, front gadja Files servis direktno.
  // Kada gateway bude gotov, ovo postaje npr. 'http://localhost:8080/api/files'.
  filesApiUrl: 'http://localhost:8082',

  // PRIVREMENO: dok Users servis/login nije gotov, saljemo test korisnika
  // u X-User-ID header-u. Kasnije ovo zamenjuje JWT iz login-a.
  devUserId: 'dev-user-1',
};
