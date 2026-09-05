//Decompile a named function; proves the native decompiler works.
//@category Test
import ghidra.app.script.GhidraScript;
import ghidra.app.decompiler.*;
import ghidra.program.model.listing.Function;

public class DecompileOne extends GhidraScript {
    @Override
    public void run() throws Exception {
        DecompInterface di = new DecompInterface();
        if (!di.openProgram(currentProgram)) {
            println("DECOMP_OPEN_FAILED: " + di.getLastMessage());
            return;
        }
        for (Function f : currentProgram.getFunctionManager().getFunctions(true)) {
            if (!f.getName().equals("add") && !f.getName().equals("main")) continue;
            DecompileResults r = di.decompileFunction(f, 30, monitor);
            if (r.decompileCompleted()) {
                println("=== " + f.getName() + " ===");
                println(r.getDecompiledFunction().getC());
            } else {
                println("DECOMP_FAILED " + f.getName() + ": " + r.getErrorMessage());
            }
        }
        di.dispose();
    }
}
