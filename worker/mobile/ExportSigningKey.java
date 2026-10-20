import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.OutputStreamWriter;
import java.io.Writer;
import java.nio.charset.StandardCharsets;
import java.security.KeyStore;
import java.security.PrivateKey;
import java.security.cert.Certificate;
import java.util.Base64;

/**
 * Exports the private key and the certificate of a keystore as PEM bundle.
 *
 * The build script signs the worker APK with the keystore of the repository. The
 * server signs the rebranded APK with the very same key, otherwise an installed
 * application cannot be updated ("App not installed"). The exported PEM is the
 * format the server reads.
 *
 * Usage: ExportSigningKey <keystore> <password> <alias> <output>
 */
public class ExportSigningKey {
    public static void main(String[] args) throws Exception {
        if (args.length < 4) {
            System.err.println("usage: ExportSigningKey <keystore> <password> <alias> <output>");
            System.exit(2);
        }

        String keystorePath = args[0];
        String password = args[1];
        String alias = args[2];
        String outputPath = args[3];

        KeyStore store = KeyStore.getInstance(KeyStore.getDefaultType());
        try (InputStream in = new FileInputStream(keystorePath)) {
            store.load(in, password.toCharArray());
        }

        PrivateKey key = (PrivateKey) store.getKey(alias, password.toCharArray());
        Certificate certificate = store.getCertificate(alias);
        if (key == null || certificate == null) {
            System.err.println("the keystore does not contain the alias " + alias);
            System.exit(1);
        }

        Base64.Encoder encoder = Base64.getMimeEncoder(64, new byte[] {'\n'});
        StringBuilder out = new StringBuilder();
        out.append("-----BEGIN PRIVATE KEY-----\n");
        out.append(encoder.encodeToString(key.getEncoded()));
        out.append("\n-----END PRIVATE KEY-----\n");
        out.append("-----BEGIN CERTIFICATE-----\n");
        out.append(encoder.encodeToString(certificate.getEncoded()));
        out.append("\n-----END CERTIFICATE-----\n");

        try (Writer writer = new OutputStreamWriter(new FileOutputStream(outputPath), StandardCharsets.UTF_8)) {
            writer.write(out.toString());
        }
    }
}
