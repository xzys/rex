//Find an Allwinner BSP DSI panel-init sequence by its CODE SHAPE rather than by name.
//
//Panel init in the sunxi disp2 BSP is not a data table -- it is a long straight-line run
//of sunxi_lcd_dsi_dcs_write_1para(sel, reg, val) calls (224 of them in k101im2qa04.c).
//Compiled for AArch64 that is a very distinctive cadence:
//    mov w1, #reg ; mov w2, #val ; bl <same callee>
//repeated hundreds of times. Nothing else in a kernel looks like that, so this works on a
//stripped blob where the panel name may not even be a recoverable string.
//
//Reports every function that calls one callee more than THRESHOLD times, then dumps the
//immediate operand pairs in order -- which IS the init sequence, ready to re-emit as C.
//
//  ...  -postScript FindDsiCadence.java [threshold]
//
//See notes/brick-display-repoint-design.md.
//@category Brick
import ghidra.app.script.GhidraScript;
import ghidra.program.model.address.Address;
import ghidra.program.model.lang.Register;
import ghidra.program.model.listing.*;
import ghidra.program.model.scalar.Scalar;
import ghidra.program.model.symbol.FlowType;

import java.util.*;

public class FindDsiCadence extends GhidraScript {

    private static final int DEFAULT_THRESHOLD = 20;

    @Override
    public void run() throws Exception {
        int threshold = DEFAULT_THRESHOLD;
        String[] args = getScriptArgs();
        if (args.length > 0) {
            try { threshold = Integer.parseInt(args[0]); } catch (NumberFormatException ignored) {}
        }
        println("CADENCE_SCAN threshold=" + threshold);

        FunctionIterator fns = currentProgram.getFunctionManager().getFunctions(true);
        while (fns.hasNext() && !monitor.isCancelled()) {
            Function f = fns.next();
            Map<String, Integer> callCounts = new HashMap<>();
            InstructionIterator it = currentProgram.getListing()
                    .getInstructions(f.getBody(), true);
            while (it.hasNext()) {
                Instruction ins = it.next();
                if (!isCall(ins)) continue;
                String tgt = calleeOf(ins);
                if (tgt != null) callCounts.merge(tgt, 1, Integer::sum);
            }
            for (Map.Entry<String, Integer> e : callCounts.entrySet()) {
                if (e.getValue() < threshold) continue;
                println("");
                println("HIT function=" + f.getName() + " @" + f.getEntryPoint()
                        + " callee=" + e.getKey() + " count=" + e.getValue());
                dumpSequence(f, e.getKey());
            }
        }
        println("CADENCE_SCAN_DONE");
    }

    private boolean isCall(Instruction ins) {
        FlowType ft = ins.getFlowType();
        return ft != null && ft.isCall();
    }

    private String calleeOf(Instruction ins) {
        Address[] flows = ins.getFlows();
        if (flows == null || flows.length == 0) return null;
        Function callee = getFunctionAt(flows[0]);
        return callee != null ? callee.getName() : flows[0].toString();
    }

    /**
     * Walk the function in address order; for every call to <callee>, report the most
     * recent immediate loaded into each argument register. That reconstructs the
     * (reg, val) pairs in the order the panel expects them.
     */
    private void dumpSequence(Function f, String callee) {
        Map<String, Long> lastImm = new HashMap<>();
        List<String> pairs = new ArrayList<>();
        InstructionIterator it = currentProgram.getListing()
                .getInstructions(f.getBody(), true);
        while (it.hasNext()) {
            Instruction ins = it.next();

            if (isCall(ins)) {
                if (callee.equals(calleeOf(ins))) {
                    Long a1 = lastImm.get("w1");
                    Long a2 = lastImm.get("w2");
                    // DCS register/value are byte-wide; the compiler emits values >= 0x80
                    // as negative immediates (movn), so mask back to a byte.
                    if (a1 != null && a2 != null) {
                        pairs.add(String.format("0x%02X 0x%02X", a1 & 0xFF, a2 & 0xFF));
                    } else if (a1 != null) {
                        pairs.add(String.format("0x%02X (1 arg)", a1 & 0xFF));
                    }
                }
                lastImm.clear();   // arguments do not survive a call
                continue;
            }

            // record "mov wN, #imm" style loads
            Object[] out = ins.getResultObjects();
            if (out.length != 1 || !(out[0] instanceof Register)) continue;
            String rn = ((Register) out[0]).getName().toLowerCase();
            if (!rn.startsWith("w") && !rn.startsWith("x")) continue;
            rn = "w" + rn.substring(1);
            Long imm = null;
            for (int i = 0; i < ins.getNumOperands(); i++) {
                for (Object o : ins.getOpObjects(i)) {
                    if (o instanceof Scalar) imm = ((Scalar) o).getUnsignedValue();
                }
            }
            if (imm != null) lastImm.put(rn, imm);
            else lastImm.remove(rn);
        }

        println("  recovered " + pairs.size() + " argument pairs:");
        StringBuilder sb = new StringBuilder("  ");
        for (int i = 0; i < pairs.size(); i++) {
            sb.append(pairs.get(i)).append("  ");
            if (i % 6 == 5) { println(sb.toString()); sb = new StringBuilder("  "); }
        }
        if (sb.length() > 2) println(sb.toString());
    }
}
